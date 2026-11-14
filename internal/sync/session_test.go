// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/test"
)

func TestSessionWaitReportsAutomaticShutdownError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	want := errors.New("server failed")
	s := &Session{
		ctx:       ctx,
		cancel:    cancel,
		discovery: &Discovery{},
		client:    &Client{},
		done:      make(chan error, 1),
	}
	s.stopped.Add(1)
	manager := &Manager{session: s}
	s.done <- want
	go func() { <-ctx.Done(); manager.stop(s) }()
	cancel()
	require.ErrorIs(t, s.Wait(), want)
}

func TestSessionRequiresUnlockedVault(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	var manager Manager
	_, err = manager.Start(context.Background(), store, nil, "Device")
	require.ErrorContains(t, err, "unlocked vault")
	require.NoError(t, manager.Stop())
}

func TestSessionTrustedFingerprints(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	vault := test.CreateVault(t, store, "Personal")
	identity := testSyncIdentity(t)
	require.NoError(t, store.AddSyncPeer(vault, storage.SyncPeer{
		PublicKey: identity.PublicKey().Bytes(),
		Name:      "Other device",
	}))

	session := &Session{store: store, vault: vault}
	fingerprints, err := session.TrustedFingerprints()
	require.NoError(t, err)
	require.Equal(t, map[string]bool{Fingerprint(identity.PublicKey()): true}, fingerprints)
}

func TestCloneSessionStartsWithoutVault(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	var manager Manager
	session, err := manager.StartClone(context.Background(), store, "Device", false)
	require.NoError(t, err)
	defer manager.Stop()
	require.NotNil(t, session.cloneKey)
	require.Nil(t, session.vault)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
}

func TestAttachCloneVaultPersistsPairedIdentity(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	identity := testSyncIdentity(t)
	remote := testSyncIdentity(t)
	session := &Session{
		ctx:      context.Background(),
		store:    store,
		cloneKey: identity,
		clonePeer: &storage.SyncPeer{
			PublicKey: remote.PublicKey().Bytes(),
			Name:      "Source",
		},
	}
	vault := test.CreateVault(t, store, "Personal")
	require.NoError(t, session.AttachCloneVault(vault))
	public, _, err := store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	require.True(t, public.Equal(identity.PublicKey()))
	peers, err := store.ListSyncPeers(vault)
	require.NoError(t, err)
	require.Len(t, peers, 1)
	require.Equal(t, remote.PublicKey().Bytes(), peers[0].PublicKey)
	require.Error(t, session.AttachCloneVault(vault))
}

func TestSessionConnectChecksFingerprintAndCancellation(t *testing.T) {
	serverIdentity := testSyncIdentity(t)
	server, err := NewServer(serverIdentity)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serverCtx, serverCancel := context.WithCancel(context.Background())
	defer serverCancel()
	done := make(chan error, 1)
	go func() { done <- server.Serve(serverCtx, listener) }()
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{ctx: ctx, cancel: cancel, client: client}
	peer := Peer{
		IP:          net.ParseIP("127.0.0.1"),
		Port:        listener.Addr().(*net.TCPAddr).Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(serverIdentity.PublicKey()),
	}
	public, err := s.Connect(context.Background(), peer)
	require.NoError(t, err)
	require.True(t, public.Equal(serverIdentity.PublicKey()))
	peer.Fingerprint = "incorrect"
	_, err = s.Connect(context.Background(), peer)
	require.ErrorContains(t, err, "fingerprint")
	cancel()
	_, err = s.Connect(context.Background(), peer)
	require.ErrorIs(t, err, context.Canceled)
	client.CloseIdleConnections()
	serverCancel()
	require.NoError(t, <-done)
}

func TestSessionConnectLinkVerifiesURIIdentity(t *testing.T) {
	identity := testSyncIdentity(t)
	server, err := NewServer(identity)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	serverCtx, stopServer := context.WithCancel(t.Context())
	t.Cleanup(stopServer)
	done := make(chan error, 1)
	go func() { done <- server.Serve(serverCtx, listener) }()
	t.Cleanup(func() { require.NoError(t, <-done) })
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	t.Cleanup(client.CloseIdleConnections)
	session := &Session{ctx: t.Context(), client: client}
	link := ConnectionLink{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, Fingerprint: Fingerprint(identity.PublicKey())}
	public, peer, err := session.ConnectLink(t.Context(), link)
	require.NoError(t, err)
	require.True(t, public.Equal(identity.PublicKey()))
	require.Equal(t, link.Fingerprint, peer.Fingerprint)
	link.Fingerprint = "0000000000000000000000000000000000000000000000000000000000000000"
	_, _, err = session.ConnectLink(t.Context(), link)
	require.ErrorContains(t, err, "fingerprint")
}

func TestSessionConnectLinkHonorsCanceledContextDuringResolution(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	session := &Session{ctx: context.Background()}
	_, _, err := session.ConnectLink(ctx, ConnectionLink{Host: "peer.invalid", Port: 1})
	require.ErrorIs(t, err, context.Canceled)
}

func TestHelloExchangesVaultReferenceOverAuthenticatedTLS(t *testing.T) {
	identity := testSyncIdentity(t)
	reference := VaultReference{Name: "Remote vault", ID: "remote-vault-id"}
	server, err := newServer(identity, nil, reference)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx, listener) }()
	t.Cleanup(func() {
		cancel()
		require.NoError(t, <-done)
	})
	client, err := NewClient(testSyncIdentity(t))
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	public, got, err := client.HelloWithVault(t.Context(), listener.Addr().String())
	require.NoError(t, err)
	require.True(t, public.Equal(identity.PublicKey()))
	require.Equal(t, reference, got)
}
