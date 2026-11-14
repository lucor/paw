// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"crypto/ed25519"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTransportAuthenticatesBothMLDSA44Peers(t *testing.T) {
	serverIdentity := testSyncIdentity(t)
	clientIdentity := testSyncIdentity(t)
	server, err := NewServer(serverIdentity)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(ctx, listener) }()

	client, err := NewClient(clientIdentity)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	peer, err := client.Hello(context.Background(), listener.Addr().String())
	require.NoError(t, err)
	require.True(t, serverIdentity.PublicKey().Equal(peer))

	response, err := client.http.Get("https://" + listener.Addr().String() + helloPath)
	require.NoError(t, err)
	require.Equal(t, uint16(tls.VersionTLS13), response.TLS.Version)
	require.Equal(t, tls.X25519MLKEM768, response.TLS.CurveID)
	require.Len(t, response.TLS.PeerCertificates, 1)
	require.NoError(t, response.Body.Close())

	response, err = client.http.Get("https://" + listener.Addr().String() + "/sync/items")
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, response.StatusCode)
	require.NoError(t, response.Body.Close())

	client.CloseIdleConnections()
	cancel()
	require.NoError(t, <-serveResult)
}

func TestSessionCertificatesRenewLazilyWithSameIdentity(t *testing.T) {
	identity := testSyncIdentity(t)
	provider, err := newSessionCertificates(identity)
	require.NoError(t, err)
	initial, err := provider.current()
	require.NoError(t, err)
	require.Same(t, initial, mustCurrentCertificate(t, provider))

	provider.mu.Lock()
	provider.certificate.Leaf.NotAfter = time.Now().Add(9 * time.Minute)
	provider.mu.Unlock()
	renewed, err := provider.current()
	require.NoError(t, err)
	require.NotSame(t, initial, renewed)
	public, err := peerIdentity([]*x509.Certificate{renewed.Leaf})
	require.NoError(t, err)
	require.True(t, identity.PublicKey().Equal(public))
	require.NotEqual(t, initial.Leaf.SerialNumber, renewed.Leaf.SerialNumber)
}

func TestSessionCertificateRenewalErrorIsReturned(t *testing.T) {
	provider, err := newSessionCertificates(testSyncIdentity(t))
	require.NoError(t, err)
	initial := provider.certificate
	provider.mu.Lock()
	provider.certificate.Leaf.NotAfter = time.Now().Add(-time.Second)
	provider.identity = nil
	provider.mu.Unlock()
	certificate, err := provider.current()
	require.ErrorContains(t, err, "ML-DSA-44 identity")
	require.Nil(t, certificate)
	require.Same(t, initial, provider.certificate)
}

func TestSessionCertificateCallbacksRenewConcurrently(t *testing.T) {
	provider, err := newSessionCertificates(testSyncIdentity(t))
	require.NoError(t, err)
	provider.mu.Lock()
	provider.certificate.Leaf.NotAfter = time.Now().Add(time.Minute)
	provider.mu.Unlock()
	const handshakes = 16
	certificates := make(chan *tls.Certificate, handshakes)
	errs := make(chan error, handshakes)
	for range handshakes {
		go func() {
			certificate, err := provider.current()
			certificates <- certificate
			errs <- err
		}()
	}
	var renewed *tls.Certificate
	for range handshakes {
		certificate := <-certificates
		require.NoError(t, <-errs)
		if renewed == nil {
			renewed = certificate
		}
		require.Same(t, renewed, certificate)
	}
}

func mustCurrentCertificate(t *testing.T, provider *sessionCertificates) *tls.Certificate {
	t.Helper()
	certificate, err := provider.current()
	require.NoError(t, err)
	return certificate
}

func TestTLSHandshakeRenewsExpiredCertificates(t *testing.T) {
	serverIdentity := testSyncIdentity(t)
	clientIdentity := testSyncIdentity(t)
	server, err := NewServer(serverIdentity)
	require.NoError(t, err)
	client, err := NewClient(clientIdentity)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	serverProvider, err := newSessionCertificates(serverIdentity)
	require.NoError(t, err)
	clientProvider, err := newSessionCertificates(clientIdentity)
	require.NoError(t, err)
	server.http.TLSConfig.GetCertificate = func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
		return serverProvider.current()
	}
	client.http.Transport.(*http.Transport).TLSClientConfig = clientTLSConfig(clientProvider)
	for _, provider := range []*sessionCertificates{serverProvider, clientProvider} {
		template := *provider.certificate.Leaf
		template.NotBefore = time.Now().Add(-2 * time.Hour)
		template.NotAfter = time.Now().Add(-time.Hour)
		der, err := x509.CreateCertificate(rand.Reader, &template, &template, provider.identity.PublicKey(), provider.identity)
		require.NoError(t, err)
		leaf, err := x509.ParseCertificate(der)
		require.NoError(t, err)
		_, err = peerIdentity([]*x509.Certificate{leaf})
		require.Error(t, err)
		expired := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: provider.identity, Leaf: leaf}
		provider.mu.Lock()
		provider.certificate = &expired
		provider.mu.Unlock()
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, listener) }()

	peer, err := client.Hello(context.Background(), listener.Addr().String())
	require.NoError(t, err)
	require.True(t, serverIdentity.PublicKey().Equal(peer))
	clientCertificate := mustCurrentCertificate(t, clientProvider)
	clientPublic, err := peerIdentity([]*x509.Certificate{clientCertificate.Leaf})
	require.NoError(t, err)
	require.True(t, clientIdentity.PublicKey().Equal(clientPublic))
	cancel()
	require.NoError(t, <-served)
}

func TestTLSResumptionStillVerifiesPeerIdentity(t *testing.T) {
	server, err := NewServer(testSyncIdentity(t))
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	served := make(chan error, 1)
	go func() { served <- server.Serve(ctx, listener) }()

	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	transport := client.http.Transport.(*http.Transport)
	transport.TLSClientConfig.ClientSessionCache = tls.NewLRUClientSessionCache(2)
	var resumedChecks atomic.Int32
	transport.TLSClientConfig.VerifyConnection = func(state tls.ConnectionState) error {
		if state.DidResume {
			resumedChecks.Add(1)
		}
		return verifyConnection(state)
	}
	for range 2 {
		response, err := client.http.Get("https://" + listener.Addr().String() + helloPath)
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		if response.TLS.DidResume {
			require.NoError(t, verifyConnection(*response.TLS))
		}
		transport.CloseIdleConnections()
	}
	require.Greater(t, resumedChecks.Load(), int32(0))
	// Reuse the TLS ticket while pinning a different identity: resumption must not bypass the pin.
	pinned := client.pairingTransport(testSyncIdentity(t).PublicKey())
	defer pinned.CloseIdleConnections()
	verifyPinned := pinned.TLSClientConfig.VerifyConnection
	var rejectedResumption atomic.Bool
	pinned.TLSClientConfig.VerifyConnection = func(state tls.ConnectionState) error {
		rejectedResumption.Store(state.DidResume)
		return verifyPinned(state)
	}
	_, err = (&http.Client{Transport: pinned, Timeout: handshakeLimit}).Get("https://" + listener.Addr().String() + helloPath)
	require.ErrorContains(t, err, "peer identity changed")
	require.True(t, rejectedResumption.Load())
	cancel()
	require.NoError(t, <-served)
}

func TestPairingRoutesUseMuxDispatch(t *testing.T) {
	server, err := newServer(testSyncIdentity(t), &pairingServer{})
	require.NoError(t, err)
	mux := server.http.Handler

	for _, test := range []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{
			name:   "pair request is registered",
			method: http.MethodPost,
			path:   pairRequestPath,
			want:   http.StatusBadRequest,
		},
		{
			name:   "pair request rejects other methods",
			method: http.MethodGet,
			path:   pairRequestPath,
			want:   http.StatusMethodNotAllowed,
		},
		{
			name:   "unknown pairing route is not registered",
			method: http.MethodPost,
			path:   "/sync/pair/unknown",
			want:   http.StatusNotFound,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, nil)
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)
			require.Equal(t, test.want, response.Code)
		})
	}
}

func TestTransportRejectsBadPeerCertificates(t *testing.T) {
	identity := testSyncIdentity(t)
	valid, err := sessionCertificate(identity)
	require.NoError(t, err)
	validCert, err := x509.ParseCertificate(valid.Certificate[0])
	require.NoError(t, err)
	_, err = peerIdentity([]*x509.Certificate{validCert})
	require.NoError(t, err)
	peerKey, err := AuthenticatedPeerKey(&http.Request{TLS: &tls.ConnectionState{
		Version: tls.VersionTLS13, CurveID: tls.X25519MLKEM768,
		PeerCertificates: []*x509.Certificate{validCert},
	}})
	require.NoError(t, err)
	require.True(t, identity.PublicKey().Equal(peerKey))

	_, edPrivate, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	now := time.Now()
	edTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "unsupported"},
		NotBefore:    now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true, IsCA: true,
	}
	edDER, err := x509.CreateCertificate(rand.Reader, edTemplate, edTemplate, edPrivate.Public(), edPrivate)
	require.NoError(t, err)
	edCert, err := x509.ParseCertificate(edDER)
	require.NoError(t, err)
	_, err = peerIdentity([]*x509.Certificate{edCert})
	require.ErrorContains(t, err, "ML-DSA-44")

	expired := *validCert
	expired.NotBefore = now.Add(-2 * time.Hour)
	expired.NotAfter = now.Add(-time.Hour)
	expiredDER, err := x509.CreateCertificate(rand.Reader, &expired, &expired, identity.PublicKey(), identity)
	require.NoError(t, err)
	expiredCert, err := x509.ParseCertificate(expiredDER)
	require.NoError(t, err)
	_, err = peerIdentity([]*x509.Certificate{expiredCert})
	require.ErrorContains(t, err, "invalid sync peer certificate")

	tamperedDER := append([]byte(nil), valid.Certificate[0]...)
	tamperedDER[len(tamperedDER)-1] ^= 1
	tamperedCert, err := x509.ParseCertificate(tamperedDER)
	require.NoError(t, err)
	_, err = peerIdentity([]*x509.Certificate{tamperedCert})
	require.ErrorContains(t, err, "invalid sync peer certificate signature")
}

func TestTransportRejectsOldTLSAndClassicalKeyExchange(t *testing.T) {
	identity := testSyncIdentity(t)
	certificate, err := sessionCertificate(identity)
	require.NoError(t, err)
	rsaCertificate := testRSACertificate(t)
	for _, test := range []struct {
		name   string
		config *tls.Config
		want   string
	}{
		{
			name:   "TLS 1.2",
			config: &tls.Config{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12, Certificates: []tls.Certificate{rsaCertificate}},
			want:   "protocol version not supported",
		},
		{
			name:   "classical key exchange",
			config: &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, CurvePreferences: []tls.CurveID{tls.X25519}, Certificates: []tls.Certificate{certificate}},
			want:   "handshake failure",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.NotFoundHandler())
			server.TLS = test.config
			server.StartTLS()
			defer server.Close()
			client, err := NewClient(testSyncIdentity(t))
			require.NoError(t, err)
			defer client.CloseIdleConnections()
			_, err = client.http.Get(server.URL)
			require.ErrorContains(t, err, test.want)
		})
	}
}

func TestServerRejectsMissingOrInvalidClientCertificate(t *testing.T) {
	serverIdentity := testSyncIdentity(t)
	for _, test := range []struct {
		name        string
		certificate tls.Certificate
		hasCert     bool
	}{
		{name: "missing"},
		{name: "unsupported key", certificate: testRSACertificate(t), hasCert: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			server, err := NewServer(serverIdentity)
			require.NoError(t, err)
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() { _ = server.Serve(ctx, listener) }()

			certificates := []tls.Certificate(nil)
			if test.hasCert {
				certificates = []tls.Certificate{test.certificate}
			}
			transport := &http.Transport{TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS13,
				MaxVersion:         tls.VersionTLS13,
				CurvePreferences:   []tls.CurveID{tls.X25519MLKEM768},
				InsecureSkipVerify: true,
				Certificates:       certificates,
			}}
			defer transport.CloseIdleConnections()
			response, err := (&http.Client{Transport: transport}).Get("https://" + listener.Addr().String() + helloPath)
			if response != nil {
				_ = response.Body.Close()
			}
			require.Error(t, err)
		})
	}
}

func TestTransportHonorsRequestCancellation(t *testing.T) {
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Hello(ctx, "127.0.0.1:1")
	require.Error(t, err)
}

func TestClientDoesNotFollowRedirects(t *testing.T) {
	identity := testSyncIdentity(t)
	certificate, err := sessionCertificate(testSyncIdentity(t))
	require.NoError(t, err)
	redirected := make(chan struct{}, 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		redirected <- struct{}{}
	}))
	defer target.Close()
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	server.TLS = &tls.Config{Certificates: []tls.Certificate{certificate}}
	server.StartTLS()
	defer server.Close()

	client, err := NewClient(identity)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	_, err = client.Hello(context.Background(), server.Listener.Addr().String())
	require.ErrorContains(t, err, "HTTP 302")
	select {
	case <-redirected:
		t.Fatal("client followed the redirect")
	default:
	}
}

func TestTransportTimesOutDuringTLSHandshake(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer conn.Close()
			<-ctx.Done()
		}
	}()
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	client.http.Transport.(*http.Transport).TLSHandshakeTimeout = 20 * time.Millisecond
	_, err = client.http.Get("https://" + listener.Addr().String())
	require.Error(t, err)
}

func TestServerClosesActiveConnectionsAfterShutdownTimeout(t *testing.T) {
	server, err := NewServer(testSyncIdentity(t))
	require.NoError(t, err)
	server.shutdownTimeout = 20 * time.Millisecond
	entered := make(chan struct{})
	server.http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-r.Context().Done()
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	serveResult := make(chan error, 1)
	go func() { serveResult <- server.Serve(ctx, listener) }()
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	requestResult := make(chan error, 1)
	go func() {
		response, err := client.http.Get("https://" + listener.Addr().String() + helloPath)
		if response != nil {
			_ = response.Body.Close()
		}
		requestResult <- err
	}()
	<-entered
	cancel()
	serveErr := <-serveResult
	require.Error(t, serveErr)
	require.True(t, errors.Is(serveErr, context.DeadlineExceeded))
	require.Error(t, <-requestResult)
}

func testSyncIdentity(t *testing.T) *mldsa.PrivateKey {
	t.Helper()
	identity, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	return identity
}

func testRSACertificate(t *testing.T) tls.Certificate {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "TLS 1.2 test"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	require.NoError(t, err)
	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}),
	)
	require.NoError(t, err)
	return certificate
}
