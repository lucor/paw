// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"crypto/mldsa"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/test"
)

func newAuthenticatedPeer(client *Client, address string, public *mldsa.PublicKey) *peer {
	return &peer{client: client, address: address, public: public}
}

func TestPairingPersistsMatchingTLSBoundTrust(t *testing.T) {
	plannerStore, plannerVault := pairingVault(t)
	defer plannerStore.Close()
	responderStore, responderVault := pairingVault(t)
	defer responderStore.Close()
	plannerPublic, plannerKey, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
	require.NoError(t, err)
	responderPublic, responderKey, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
	require.NoError(t, err)
	remote, incoming := startPairingEndpoint(
		t,
		responderStore,
		responderVault,
		responderPublic,
		responderKey,
		"Responder",
	)
	plannerClient, err := NewClient(plannerKey)
	require.NoError(t, err)
	defer plannerClient.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	plannerReference := VaultReference{Name: "Planner", ID: plannerVault.Accounts[0].ID}
	session := &Session{
		ctx:        ctx,
		client:     plannerClient,
		store:      plannerStore,
		vault:      plannerVault,
		public:     plannerPublic,
		vaultRef:   plannerReference,
		deviceName: "Planner",
	}
	peer := Peer{
		IP:          remote.IP,
		Port:        remote.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(responderPublic),
	}
	result := make(chan *Pairing, 1)
	errResult := make(chan error, 1)
	go func() {
		pairing, err := session.RequestPairing(context.Background(), peer)
		if err != nil {
			errResult <- err
			return
		}
		result <- pairing
	}()
	var request PairingRequest
	select {
	case request = <-incoming:
	case err := <-errResult:
		t.Fatalf("planner pairing request failed: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("pairing request was not delivered")
	}
	require.Equal(t, "Planner", request.Name)
	require.Equal(t, plannerReference, request.Vault)
	remoteCode, err := request.VerificationCode()
	require.NoError(t, err)
	var pairing *Pairing
	select {
	case pairing = <-result:
	case err := <-errResult:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("planner did not receive pairing response")
	}
	require.Equal(t, remoteCode, pairing.Code())
	address := net.JoinHostPort(remote.IP.String(), strconv.Itoa(remote.Port))
	wrongCode := strings.Repeat("0", 6)
	if wrongCode == remoteCode {
		wrongCode = strings.Repeat("1", 6)
	}
	require.ErrorContains(t, pairing.Confirm(context.Background(), wrongCode), "does not match")
	err = newAuthenticatedPeer(plannerClient, address, responderPublic).pairingAction(
		context.Background(),
		"/sync/pair/confirm",
		pairingAction{
			ID:   pairing.id,
			Code: wrongCode,
		},
	)
	require.ErrorContains(t, err, "HTTP 400")
	waitCtx, waitCancel := context.WithCancel(context.Background())
	waitCancel()
	require.ErrorIs(t, request.Wait(waitCtx), context.Canceled)
	confirmed := make(chan error, 1)
	go func() { confirmed <- pairing.Confirm(context.Background(), pairing.Code()) }()
	select {
	case err := <-confirmed:
		t.Fatalf("planner confirmation did not wait for the responder: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	require.NoError(t, request.Accept())
	require.NoError(t, awaitPairing(t, confirmed))
	require.NoError(t, request.Wait(context.Background()))
	require.ErrorIs(t, pairing.Cancel(context.Background()), errPairingCompleted)
	require.ErrorIs(t, request.Cancel(), errPairingCompleted)

	plannerPeers, err := plannerStore.ListSyncPeers(plannerVault)
	require.NoError(t, err)
	responderPeers, err := responderStore.ListSyncPeers(responderVault)
	require.NoError(t, err)
	require.Len(t, plannerPeers, 1)
	require.Len(t, responderPeers, 1)
	require.Equal(t, "Responder", plannerPeers[0].Name)
	require.Equal(t, "Planner", responderPeers[0].Name)
	require.Equal(t, responderPublic.Bytes(), plannerPeers[0].PublicKey)
	require.Equal(t, plannerPublic.Bytes(), responderPeers[0].PublicKey)
	require.Equal(t, "Planner", <-request.peer.syncRequests)
	otherIdentity := testSyncIdentity(t)
	require.NoError(t, responderStore.AddSyncPeer(responderVault, storage.SyncPeer{
		PublicKey: otherIdentity.PublicKey().Bytes(),
		Name:      "Other trusted device",
	}))
	otherClient, err := NewClient(otherIdentity)
	require.NoError(t, err)
	defer otherClient.CloseIdleConnections()
	require.ErrorContains(t, newAuthenticatedPeer(otherClient, address, responderPublic).syncComplete(context.Background(), true, false), "HTTP 403")
	outsiderClient, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer outsiderClient.CloseIdleConnections()
	address = net.JoinHostPort(remote.IP.String(), strconv.Itoa(remote.Port))
	require.ErrorContains(t, newAuthenticatedPeer(outsiderClient, address, responderPublic).syncComplete(context.Background(), true, false), "HTTP 403")
	select {
	case <-request.peer.syncOutcomes:
		t.Fatal("untrusted peer completed the Sync session")
	default:
	}
	require.NoError(t, session.NotifySyncComplete(context.Background(), peer, true))
	require.True(t, <-request.peer.syncOutcomes)
	require.ErrorContains(t, session.NotifySyncComplete(context.Background(), peer, true), "HTTP 409")
	_, err = session.RequestPairing(context.Background(), peer)
	require.ErrorContains(t, err, "already paired")

	trusted, err := session.ConnectTrusted(context.Background(), peer)
	require.NoError(t, err)
	require.True(t, trusted.Equal(responderPublic))
	changedIdentity, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	changedPublic := changedIdentity.PublicKey()
	changedAddress, _ := startPairingEndpoint(
		t,
		responderStore,
		responderVault,
		changedPublic,
		changedIdentity,
		"Changed",
	)
	changedPeer := Peer{
		IP:          changedAddress.IP,
		Port:        changedAddress.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(changedPublic),
	}
	changedHost := net.JoinHostPort(changedAddress.IP.String(), strconv.Itoa(changedAddress.Port))
	_, err = newAuthenticatedPeer(plannerClient, changedHost, responderPublic).pair(
		context.Background(),
		pairingRequest{
			Name:  "Planner",
			Nonce: strings.Repeat("0", 64),
		},
		plannerPublic,
	)
	require.ErrorContains(t, err, "identity changed")
	err = newAuthenticatedPeer(plannerClient, changedHost, responderPublic).pairingAction(
		context.Background(),
		"/sync/pair/confirm",
		pairingAction{ID: "unused"},
	)
	require.ErrorContains(t, err, "identity changed")
	_, err = session.ConnectTrusted(context.Background(), changedPeer)
	require.ErrorContains(t, err, "not paired")
	require.NoError(t, plannerStore.RemoveSyncPeer(plannerVault, responderPublic.Bytes()))
	_, err = session.ConnectTrusted(context.Background(), peer)
	require.ErrorContains(t, err, "not paired")
}

func TestIsPairingCancellationUsesHTTPStatus(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusGone, http.StatusNotFound} {
		require.True(t, isPairingCancellation(&pairingHTTPError{status: status, message: "unrelated wording"}))
	}
	for _, status := range []int{http.StatusBadRequest, http.StatusConflict, http.StatusRequestTimeout, http.StatusServiceUnavailable} {
		require.False(t, isPairingCancellation(&pairingHTTPError{status: status, message: "pairing declined"}))
	}
	require.False(t, isPairingCancellation(context.DeadlineExceeded))
}

func TestReadPairingRequestValidatesDeviceName(t *testing.T) {
	for _, test := range []struct {
		name string
		body string
		want string
	}{
		{name: "valid", body: `{"name":"Device","nonce":"nonce"}`},
		{name: "missing name", body: `{"nonce":"nonce"}`, want: "device name is required"},
		{name: "empty name", body: `{"name":"","nonce":"nonce"}`, want: "device name is required"},
		{name: "long name", body: `{"name":"` + strings.Repeat("x", 256) + `","nonce":"nonce"}`, want: "device name is too long"},
		{name: "surrounding whitespace", body: `{"name":" Device ","nonce":"nonce"}`, want: "device name has surrounding whitespace"},
		{name: "invalid JSON", body: `{`, want: "unexpected"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, err := readPairingRequest(strings.NewReader(test.body))
			if test.want != "" {
				require.ErrorContains(t, err, test.want)
				return
			}
			require.NoError(t, err)
			require.Equal(t, "Device", request.Name)
			require.Equal(t, "nonce", request.Nonce)
		})
	}
}

func TestPairingRequestRejectsMalformedNonce(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	serverIdentity := testSyncIdentity(t)
	clientIdentity := testSyncIdentity(t)
	certificate, err := sessionCertificate(clientIdentity)
	require.NoError(t, err)
	peerCertificate, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	server := newPairingServer(store, vault, "Local", serverIdentity.PublicKey())
	request := httptest.NewRequest(http.MethodPost, pairRequestPath, strings.NewReader(
		`{"name":"Remote","nonce":"not-hex"}`,
	))
	request.TLS = &tls.ConnectionState{
		Version:          tls.VersionTLS13,
		CurveID:          tls.X25519MLKEM768,
		PeerCertificates: []*x509.Certificate{peerCertificate},
	}
	response := httptest.NewRecorder()
	server.request(response, request)
	require.Equal(t, http.StatusBadRequest, response.Code)
	require.Contains(t, response.Body.String(), "invalid pairing nonce")
}

func TestPairingConfirmRejectsDifferentPeerKey(t *testing.T) {
	expected := testSyncIdentity(t)
	other := testSyncIdentity(t)
	certificate, err := sessionCertificate(other)
	require.NoError(t, err)
	peerCertificate, err := x509.ParseCertificate(certificate.Certificate[0])
	require.NoError(t, err)
	state := &pairingState{
		public:   expected.PublicKey(),
		code:     "123456",
		decision: make(chan bool, 1),
		done:     make(chan struct{}),
	}
	server := &pairingServer{pending: map[string]*pairingState{"request": state}}
	request := httptest.NewRequest(http.MethodPost, pairConfirmPath, strings.NewReader(
		`{"id":"request","code":"123456"}`,
	))
	request.TLS = &tls.ConnectionState{
		Version:          tls.VersionTLS13,
		CurveID:          tls.X25519MLKEM768,
		PeerCertificates: []*x509.Certificate{peerCertificate},
	}
	response := httptest.NewRecorder()
	server.confirm(response, request)
	require.Equal(t, http.StatusNotFound, response.Code)
	require.False(t, state.writing)
}

func TestPairingRequestRejectsConcurrentRequest(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	plannerIdentity := testSyncIdentity(t)
	responderIdentity, responderKey, err := store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	address, incoming := startPairingEndpoint(
		t,
		store,
		vault,
		responderIdentity,
		responderKey,
		"Responder",
	)
	client, err := NewClient(plannerIdentity)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	remote := net.JoinHostPort(address.IP.String(), strconv.Itoa(address.Port))
	peer := newAuthenticatedPeer(client, remote, responderIdentity)
	firstResult := make(chan error, 1)
	go func() {
		_, err := peer.pair(context.Background(), pairingRequest{
			Name:  "Planner",
			Nonce: strings.Repeat("0", 64),
		}, plannerIdentity.PublicKey())
		firstResult <- err
	}()
	request := awaitPairing(t, incoming)
	_, err = peer.pair(context.Background(), pairingRequest{
		Name:  "Planner",
		Nonce: strings.Repeat("1", 64),
	}, plannerIdentity.PublicKey())
	require.ErrorContains(t, err, "HTTP 409")
	require.NoError(t, request.Decline())
	require.NoError(t, <-firstResult)
}

func TestPairingRejectsOversizedSuccessfulResponse(t *testing.T) {
	planner := testSyncIdentity(t)
	responder := testSyncIdentity(t)
	endpoint := pairingTestServer(t, responder, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{}`+strings.Repeat(" ", pairingResponseLimit))
	}))
	client, err := NewClient(planner)
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	peer := newAuthenticatedPeer(client, endpoint.Listener.Addr().String(), responder.PublicKey())
	_, err = peer.pair(context.Background(), pairingRequest{Name: "Planner", Nonce: strings.Repeat("0", 64)}, planner.PublicKey())
	require.ErrorContains(t, err, "exceeds 4096 bytes")
}

func TestSyncCompletionReportsFailureWithoutSuccess(t *testing.T) {
	store, vault := pairingVault(t)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	plannerIdentity := testSyncIdentity(t)
	responderIdentity := testSyncIdentity(t)
	responder := newPairingServer(store, vault, "Responder", responderIdentity.PublicKey())
	require.NoError(t, store.AddSyncPeer(vault, storage.SyncPeer{
		PublicKey: plannerIdentity.PublicKey().Bytes(),
		Name:      "Planner",
	}))
	require.True(t, responder.beginSync(plannerIdentity.PublicKey(), "Planner"))
	server, err := newServer(responderIdentity, responder)
	require.NoError(t, err)
	endpoint := pairingTestServer(t, responderIdentity, server.http.Handler)
	client, err := NewClient(plannerIdentity)
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)

	err = newAuthenticatedPeer(client, endpoint.Listener.Addr().String(), responderIdentity.PublicKey()).syncComplete(context.Background(), false, false)

	require.NoError(t, err)
	require.False(t, <-responder.syncOutcomes)
	peers, err := store.ListSyncPeers(vault)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.Equal(t, plannerIdentity.PublicKey().Bytes(), peers[0].PublicKey)
}

func TestSyncCancellationIsReportedOnce(t *testing.T) {
	store, vault := pairingVault(t)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	planner := testSyncIdentity(t)
	identity := testSyncIdentity(t)
	responder := newPairingServer(store, vault, "Responder", identity.PublicKey())
	t.Cleanup(responder.clear)
	require.NoError(t, store.AddSyncPeer(vault, storage.SyncPeer{
		PublicKey: planner.PublicKey().Bytes(),
		Name:      "Planner",
	}))
	require.True(t, responder.beginSync(planner.PublicKey(), "Planner"))
	server, err := newServer(identity, responder)
	require.NoError(t, err)
	endpoint := pairingTestServer(t, identity, server.http.Handler)
	client, err := NewClient(planner)
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	remote := newAuthenticatedPeer(client, endpoint.Listener.Addr().String(), identity.PublicKey())
	require.NoError(t, remote.syncCancelled(context.Background(), false))
	require.False(t, <-responder.syncOutcomes)
	responder.mu.Lock()
	cancelled := responder.outcomeCancelled
	responder.mu.Unlock()
	require.True(t, cancelled)
	require.ErrorContains(t, remote.syncCancelled(context.Background(), false), "HTTP 409")
}

func TestSyncHeartbeatExpiryReportsUnknownOnce(t *testing.T) {
	identity := testSyncIdentity(t)
	responder := newPairingServer(nil, nil, "Responder", identity.PublicKey())
	responder.activePeer = identity.PublicKey()
	responder.syncDeadline = time.Now().Add(-time.Second)
	responder.expireSync()
	responder.expireSync()
	select {
	case <-responder.syncLost:
	default:
		t.Fatal("expected connection loss")
	}
	select {
	case <-responder.syncLost:
		t.Fatal("connection loss emitted twice")
	case <-responder.syncOutcomes:
		t.Fatal("connection loss must not be reported as completion")
	default:
	}
}

func TestSyncPlannerDisconnectReportsUnknownToResponder(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	plannerStore, plannerVault := pairingVault(t)
	defer plannerStore.Close()
	responderStore, responderVault := pairingVault(t)
	defer responderStore.Close()
	plannerPublic, _, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
	require.NoError(t, err)
	responderPublic, _, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
	require.NoError(t, err)
	require.NoError(t, plannerStore.AddSyncPeer(plannerVault, storage.SyncPeer{
		PublicKey: responderPublic.Bytes(),
		Name:      "Responder",
	}))
	require.NoError(t, responderStore.AddSyncPeer(responderVault, storage.SyncPeer{
		PublicKey: plannerPublic.Bytes(),
		Name:      "Planner",
	}))
	var plannerManager, responderManager Manager
	planner, err := plannerManager.StartManual(ctx, plannerStore, plannerVault, "Planner")
	require.NoError(t, err)
	defer plannerManager.Stop()
	responder, err := responderManager.StartManual(ctx, responderStore, responderVault, "Responder")
	require.NoError(t, err)
	defer responderManager.Stop()
	peer := Peer{
		IP:          net.ParseIP("127.0.0.1"),
		Port:        responder.Port(),
		Version:     protocolVersion,
		Fingerprint: Fingerprint(responderPublic),
	}
	_, err = planner.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	select {
	case <-responder.IncomingSyncs():
	case <-ctx.Done():
		t.Fatal("responder did not start waiting")
	}
	require.NoError(t, plannerManager.Stop())
	responder.pairing.mu.Lock()
	responder.pairing.syncDeadline = time.Now().Add(-time.Second)
	responder.pairing.mu.Unlock()
	responder.pairing.expireSync()
	select {
	case <-responder.SyncConnectionLost():
	case <-ctx.Done():
		t.Fatal("responder did not report connection loss")
	}
	select {
	case <-responder.SyncOutcomes():
		t.Fatal("connection loss must not report a completion")
	default:
	}
}

func TestPairingDeclineDoesNotPersistTrust(t *testing.T) {
	plannerStore, plannerVault := pairingVault(t)
	defer plannerStore.Close()
	responderStore, responderVault := pairingVault(t)
	defer responderStore.Close()
	plannerPublic, plannerKey, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
	require.NoError(t, err)
	responderPublic, responderKey, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
	require.NoError(t, err)
	remote, incoming := startPairingEndpoint(
		t,
		responderStore,
		responderVault,
		responderPublic,
		responderKey,
		"Responder",
	)
	client, err := NewClient(plannerKey)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &Session{
		ctx:        ctx,
		client:     client,
		store:      plannerStore,
		vault:      plannerVault,
		public:     plannerPublic,
		deviceName: "Planner",
	}
	peer := Peer{
		IP:          remote.IP,
		Port:        remote.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(responderPublic),
	}
	result := make(chan *Pairing, 1)
	errResult := make(chan error, 1)
	go func() {
		pairing, err := session.RequestPairing(context.Background(), peer)
		if err != nil {
			errResult <- err
			return
		}
		result <- pairing
	}()
	var request PairingRequest
	select {
	case request = <-incoming:
	case err := <-errResult:
		t.Fatalf("planner pairing request failed: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("pairing request was not delivered")
	}
	var pairing *Pairing
	select {
	case pairing = <-result:
	case <-time.After(10 * time.Second):
		t.Fatal("planner did not receive the verification code")
	}
	require.NoError(t, request.Decline())
	require.ErrorIs(t, pairing.Confirm(context.Background(), pairing.Code()), errPairingCancelled)
	plannerPeers, err := plannerStore.ListSyncPeers(plannerVault)
	require.NoError(t, err)
	responderPeers, err := responderStore.ListSyncPeers(responderVault)
	require.NoError(t, err)
	require.Empty(t, plannerPeers)
	require.Empty(t, responderPeers)
}

func TestPairingCancelDoesNotPersistTrust(t *testing.T) {
	plannerStore, plannerVault := pairingVault(t)
	defer plannerStore.Close()
	responderStore, responderVault := pairingVault(t)
	defer responderStore.Close()
	plannerPublic, plannerKey, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
	require.NoError(t, err)
	responderPublic, responderKey, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
	require.NoError(t, err)
	remote, incoming := startPairingEndpoint(
		t,
		responderStore,
		responderVault,
		responderPublic,
		responderKey,
		"Responder",
	)
	client, err := NewClient(plannerKey)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	session := &Session{
		ctx:        ctx,
		client:     client,
		store:      plannerStore,
		vault:      plannerVault,
		public:     plannerPublic,
		deviceName: "Planner",
	}
	peer := Peer{
		IP:          remote.IP,
		Port:        remote.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(responderPublic),
	}
	result := make(chan *Pairing, 1)
	errResult := make(chan error, 1)
	go func() {
		pairing, err := session.RequestPairing(context.Background(), peer)
		if err != nil {
			errResult <- err
			return
		}
		result <- pairing
	}()
	var request PairingRequest
	select {
	case request = <-incoming:
	case err := <-errResult:
		t.Fatalf("planner pairing request failed: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("pairing request was not delivered")
	}
	require.NoError(t, request.Accept())
	var pairing *Pairing
	select {
	case pairing = <-result:
	case err := <-errResult:
		t.Fatalf("planner did not receive pairing response: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("planner did not receive pairing response")
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	require.NoError(t, pairing.Cancel(canceled))
	require.Empty(t, pairing.Code())
	require.ErrorIs(t, pairing.Confirm(context.Background(), "000000"), errPairingCancelled)
	require.NoError(t, request.Cancel())
	require.ErrorIs(t, request.Wait(context.Background()), errPairingCancelled)
	require.Error(t, request.Accept())
	require.NoError(t, pairing.Cancel(context.Background()))
	plannerPeers, err := plannerStore.ListSyncPeers(plannerVault)
	require.NoError(t, err)
	responderPeers, err := responderStore.ListSyncPeers(responderVault)
	require.NoError(t, err)
	require.Empty(t, plannerPeers)
	require.Empty(t, responderPeers)
}

func TestPairingFinalizationInterruption(t *testing.T) {
	for _, tc := range []struct {
		name   string
		path   string
		saved  bool
		cancel bool
	}{
		{"cancel during confirmation", "/sync/pair/confirm", false, true},
		{"cancel during final acknowledgement", "/sync/pair/complete", true, true},
		{"lost final acknowledgement", "/sync/pair/complete", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store, vault := pairingVault(t)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			plannerKey, err := mldsa.GenerateKey(mldsa.MLDSA44())
			require.NoError(t, err)
			responderKey, err := mldsa.GenerateKey(mldsa.MLDSA44())
			require.NoError(t, err)
			public := responderKey.PublicKey()
			reached := make(chan struct{})
			release := make(chan struct{})
			var releaseOnce sync.Once
			unblock := func() { releaseOnce.Do(func() { close(release) }) }
			endpoint := pairingTestServer(t, responderKey, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == tc.path {
					close(reached)
					select {
					case <-release:
						http.Error(w, "unavailable", http.StatusServiceUnavailable)
					case <-r.Context().Done():
					}
					return
				}
				if r.URL.Path == "/sync/pair/cancel" {
					return
				}
				_, _ = w.Write([]byte(`{"stored":true}`))
			}))
			t.Cleanup(unblock)
			client, err := NewClient(plannerKey)
			require.NoError(t, err)
			t.Cleanup(client.CloseIdleConnections)
			pairing := &Pairing{
				session: &Session{
					ctx:    t.Context(),
					client: client,
					store:  store,
					vault:  vault,
				},
				peer:   newAuthenticatedPeer(client, endpoint.Listener.Addr().String(), public),
				id:     "test",
				code:   "123456",
				name:   "Responder",
				public: public,
			}
			result := make(chan error, 1)
			go func() { result <- pairing.Confirm(t.Context(), "123456") }()
			awaitPairing(t, reached)
			require.ErrorContains(t, pairing.Confirm(t.Context(), "123456"), "in progress")
			want := errPairingInterrupted
			if tc.cancel {
				cancelled := make(chan error, 1)
				go func() { cancelled <- pairing.Cancel(t.Context()) }()
				require.NoError(t, awaitPairing(t, cancelled))
				want = errPairingCancelled
			} else {
				unblock()
			}
			require.ErrorIs(t, awaitPairing(t, result), want)
			require.ErrorIs(t, pairing.Confirm(t.Context(), "123456"), want)
			unblock()
			endpoint.Close()
			require.NoError(t, pairing.Cancel(t.Context()))
			peers, err := pairing.session.PairedPeers()
			require.NoError(t, err)
			if tc.saved {
				require.Len(t, peers, 1)
				require.Equal(t, public.Bytes(), peers[0].PublicKey)
				require.NoError(t, pairing.session.RemovePairedPeer(public.Bytes()))
				peers, err = pairing.session.PairedPeers()
				require.NoError(t, err)
			}
			require.Empty(t, peers)
		})
	}
}

type blockedPairingStore struct {
	*storage.FileStorage
	started chan struct{}
	release chan struct{}
}

func (s *blockedPairingStore) AddSyncPeer(vault *model.Vault, peer storage.SyncPeer) error {
	close(s.started)
	<-s.release
	return s.FileStorage.AddSyncPeer(vault, peer)
}

func TestPairingCancelLetsAdmittedTrustWriteFinish(t *testing.T) {
	store, vault := pairingVault(t)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	plannerKey, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	responderKey, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	public := responderKey.PublicKey()
	server := newPairingServer(store, vault, "Responder", public)
	blocked := &blockedPairingStore{
		FileStorage: store,
		started:     make(chan struct{}),
		release:     make(chan struct{}),
	}
	server.store = blocked
	state := &pairingState{
		name:     "Planner",
		public:   plannerKey.PublicKey(),
		code:     "123456",
		decision: make(chan bool, 1),
		done:     make(chan struct{}),
	}
	state.decision <- true
	server.pending["test"] = state
	httpServer, err := newServer(responderKey, server)
	require.NoError(t, err)
	endpoint := pairingTestServer(t, responderKey, httpServer.http.Handler)
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(blocked.release) }) }
	t.Cleanup(unblock)
	client, err := NewClient(plannerKey)
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	address := endpoint.Listener.Addr().String()
	action := pairingAction{ID: "test", Code: "123456"}
	result := make(chan error, 1)
	go func() {
		result <- newAuthenticatedPeer(client, address, public).pairingAction(t.Context(), "/sync/pair/confirm", action)
	}()
	awaitPairing(t, blocked.started)
	request := PairingRequest{
		ID:    "test",
		peer:  server,
		state: state,
	}
	cancelled := make(chan error, 1)
	go func() { cancelled <- request.Cancel() }()
	require.NoError(t, awaitPairing(t, cancelled))
	require.ErrorIs(t, request.Wait(t.Context()), errPairingCancelled)
	unblock()
	require.ErrorContains(t, awaitPairing(t, result), "pairing cancelled")
	peers, err := store.ListSyncPeers(vault)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.Equal(t, plannerKey.PublicKey().Bytes(), peers[0].PublicKey)
	require.Error(t, newAuthenticatedPeer(client, address, public).pairingAction(t.Context(), "/sync/pair/complete", action))
}

func pairingTestServer(t *testing.T, key *mldsa.PrivateKey, handler http.Handler) *httptest.Server {
	t.Helper()
	server, err := NewServer(key)
	require.NoError(t, err)
	endpoint := httptest.NewUnstartedServer(handler)
	endpoint.TLS = server.http.TLSConfig
	// StartTLS installs a default certificate when Certificates is empty, even with GetCertificate.
	// Seed it with the Sync certificate so the helper never presents its default ECDSA identity.
	certificate, err := endpoint.TLS.GetCertificate(nil)
	require.NoError(t, err)
	endpoint.TLS.Certificates = []tls.Certificate{*certificate}
	endpoint.StartTLS()
	t.Cleanup(endpoint.Close)
	return endpoint
}

func awaitPairing[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for pairing test")
		var zero T
		return zero
	}
}

func pairingVault(t *testing.T) (*storage.FileStorage, *model.Vault) {
	t.Helper()
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	vault := test.CreateVault(t, store, "Test")
	return store, vault
}

func startPairingEndpoint(t *testing.T, store *storage.FileStorage, vault *model.Vault, public *mldsa.PublicKey, key *mldsa.PrivateKey, name string) (*net.TCPAddr, <-chan PairingRequest) {
	return startPairingEndpointWithStore(t, store, store, vault, public, key, name)
}

func startPairingEndpointWithStore(t *testing.T, base *storage.FileStorage, store pairingTrustStore, vault *model.Vault, public *mldsa.PublicKey, key *mldsa.PrivateKey, name string) (*net.TCPAddr, <-chan PairingRequest) {
	address, incoming, _ := startPairingEndpointWithProgress(t, base, store, vault, public, key, name)
	return address, incoming
}

func startPairingEndpointWithProgress(t *testing.T, base *storage.FileStorage, store pairingTrustStore, vault *model.Vault, public *mldsa.PublicKey, key *mldsa.PrivateKey, name string) (*net.TCPAddr, <-chan PairingRequest, <-chan SyncProgressEvent) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	pairing := newPairingServer(base, vault, name, public)
	pairing.store = store
	server, err := newServer(key, pairing)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.Serve(ctx, listener)
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("pairing endpoint did not stop")
		}
	})
	return listener.Addr().(*net.TCPAddr), pairing.incoming, pairing.syncProgress
}

func TestReleaseConfirmationAllowsRetry(t *testing.T) {
	t.Run("pending pairing releases the confirmation", func(t *testing.T) {
		state := &pairingState{writing: true}
		server := &pairingServer{pending: map[string]*pairingState{"id": state}}
		server.releaseConfirmation("id", state)
		require.False(t, state.writing)
	})
	t.Run("finished pairing stays closed", func(t *testing.T) {
		state := &pairingState{writing: true, finished: true}
		server := &pairingServer{pending: map[string]*pairingState{"id": state}}
		server.releaseConfirmation("id", state)
		require.True(t, state.writing)
	})
}
