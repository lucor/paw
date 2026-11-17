// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"time"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/netif"
	"go.lucor.dev/paw/internal/storage"
)

// Manager owns the single active Sync session of an application instance.
type Manager struct {
	mu      sync.Mutex
	session *Session
}

// Session represents one unlocked vault's temporary LAN availability.
// Its methods must be called from the goroutine that started it: peer, vault,
// and pairing fields are not synchronized with the server handler goroutines.
type Session struct {
	ctx                  context.Context
	cancel               context.CancelFunc
	discovery            *Discovery
	client               *Client
	store                *storage.FileStorage
	vault                *model.Vault
	public               *mldsa.PublicKey
	cloneKey             *mldsa.PrivateKey
	clonePeer            *storage.SyncPeer
	peer                 *peer
	peerVault            VaultReference
	vaultRef             VaultReference
	deviceName           string
	clone                bool
	cloneHeartbeatCancel context.CancelFunc
	syncHeartbeatCancel  context.CancelFunc
	pairing              *pairingServer
	done                 chan error
	stopped              sync.WaitGroup
	stopErr              error
}

// Start makes one unlocked vault available until ctx is canceled or Stop is called.
// The caller must cancel ctx when the vault locks.
func (m *Manager) Start(ctx context.Context, store *storage.FileStorage, vault *model.Vault, deviceName string) (*Session, error) {
	return m.start(ctx, store, vault, deviceName, true, false)
}

// StartManual starts a Sync listener without mDNS discovery for a direct link connection.
func (m *Manager) StartManual(ctx context.Context, store *storage.FileStorage, vault *model.Vault, deviceName string) (*Session, error) {
	return m.start(ctx, store, vault, deviceName, false, false)
}

// StartClone starts a temporary initiating session without a local vault.
func (m *Manager) StartClone(ctx context.Context, store *storage.FileStorage, deviceName string, discover bool) (*Session, error) {
	return m.start(ctx, store, nil, deviceName, discover, true)
}

// AttachCloneVault persists the temporary identity and trust in a newly created vault.
func (s *Session) AttachCloneVault(vault *model.Vault) error {
	if s == nil || s.cloneKey == nil || s.clonePeer == nil || s.vault != nil {
		return fmt.Errorf("a paired clone session is required")
	}
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if err := s.store.StoreSyncIdentity(vault, s.cloneKey); err != nil {
		return err
	}
	if err := s.store.AddSyncPeer(vault, *s.clonePeer); err != nil {
		return err
	}
	s.vault = vault
	s.cloneKey = nil
	return nil
}

func (m *Manager) start(ctx context.Context, store *storage.FileStorage, vault *model.Vault, deviceName string, discover, clone bool) (*Session, error) {
	logger := slog.With("component", "sync.session")
	logger.Debug("starting Sync session", "device", deviceName)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		return nil, fmt.Errorf("sync is already active")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("unlocked vault and storage are required")
	}
	if !clone && vault == nil {
		return nil, fmt.Errorf("unlocked vault and storage are required")
	}
	if !clone && vault.Key() == nil {
		return nil, fmt.Errorf("unlocked vault and storage are required")
	}
	if deviceName == "" {
		return nil, fmt.Errorf("device name is required")
	}
	var public *mldsa.PublicKey
	var private *mldsa.PrivateKey
	var err error
	if clone {
		private, err = mldsa.GenerateKey(mldsa.MLDSA44())
		if err == nil {
			public = private.PublicKey()
		}
	} else {
		public, private, err = store.LoadOrCreateSyncIdentity(vault)
	}
	if err != nil {
		return nil, err
	}
	pairing := newPairingServer(store, vault, deviceName, public)
	var vaultRef VaultReference
	if !clone {
		vaultName, err := vault.Name()
		if err != nil {
			return nil, err
		}
		vaultRef = VaultReference{Name: vaultName, ID: vault.Accounts[0].ID}
	}
	pairing.vaultRef = vaultRef
	serverPairing := pairing
	if clone {
		serverPairing = nil
	}
	server, err := newServer(private, serverPairing, vaultRef)
	if err != nil {
		return nil, err
	}
	client, err := NewClient(private)
	if err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		logger.Warn("could not start Sync listener", "error", err)
		client.CloseIdleConnections()
		return nil, err
	}
	var token [16]byte
	if _, err = rand.Read(token[:]); err != nil {
		_ = listener.Close()
		client.CloseIdleConnections()
		return nil, err
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	discovery := NewDiscovery(
		"paw-"+hex.EncodeToString(token[:]),
		deviceName,
		listener.Addr().(*net.TCPAddr).Port,
		Fingerprint(public),
	)
	if discover {
		if clone {
			err = discovery.StartBrowsing(sessionCtx)
		} else {
			err = discovery.Start(sessionCtx)
		}
	}
	if err != nil {
		cancel()
		_ = listener.Close()
		client.CloseIdleConnections()
		return nil, err
	}
	s := &Session{
		ctx:        sessionCtx,
		cancel:     cancel,
		discovery:  discovery,
		client:     client,
		store:      store,
		vault:      vault,
		vaultRef:   vaultRef,
		public:     public,
		deviceName: deviceName,
		clone:      clone,
		pairing:    pairing,
		done:       make(chan error, 1),
	}
	if clone {
		s.cloneKey = private
	}
	s.stopped.Add(1)
	m.session = s
	logger.Debug("Sync HTTPS listener started", "address", listener.Addr().String(), "port", listener.Addr().(*net.TCPAddr).Port)
	go func() {
		err := server.Serve(sessionCtx, listener)
		_ = listener.Close()
		s.done <- err
		cancel()
	}()
	go func() {
		<-sessionCtx.Done()
		m.stop(s)
	}()
	return s, nil
}

// RequestPairing requests an explicitly initiated pairing with a discovered peer.
// It returns the TLS-bound code before the responder makes a decision.
func (s *Session) RequestPairing(ctx context.Context, peer Peer) (*Pairing, error) {
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	pairingCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(s.ctx, cancel)
	defer func() {
		stop()
		cancel()
	}()
	return s.requestPairing(pairingCtx, peer)
}

// Fingerprint identifies an ML-DSA public key for discovery consistency checks.
func Fingerprint(public *mldsa.PublicKey) string {
	sum := sha256.Sum256(public.Bytes())
	return hex.EncodeToString(sum[:])
}

// Peers reports untrusted discovery results.
func (s *Session) Peers() <-chan Peer {
	return s.discovery.Peers()
}

// Wait returns the session's shutdown error after its resources are released.
func (s *Session) Wait() error {
	s.stopped.Wait()
	return s.stopErr
}

// Port returns the dynamically assigned HTTPS listening port.
func (s *Session) Port() int {
	return s.discovery.port
}

// ConnectionLink returns a link for the active listener using a LAN address.
func (s *Session) ConnectionLink() (string, error) {
	interfaces, err := netif.Snapshot()
	if err != nil {
		return "", err
	}
	for _, item := range interfaces {
		iface := item.Interface
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		for _, address := range item.Addrs {
			ipNet, ok := address.(*net.IPNet)
			if !ok || !ipNet.IP.IsGlobalUnicast() || ipNet.IP.IsLinkLocalUnicast() {
				continue
			}
			return ConnectionLink{
				Host:        ipNet.IP.String(),
				Port:        s.Port(),
				Fingerprint: Fingerprint(s.public),
			}.String(), nil
		}
	}
	return "", fmt.Errorf("no connectable network address is available")
}

// NotifySyncComplete sends the outcome to the selected trusted peer.
func (s *Session) NotifySyncComplete(ctx context.Context, peer Peer, successful bool) error {
	s.stopSyncHeartbeat()
	if s.cloneHeartbeatCancel != nil {
		s.cloneHeartbeatCancel()
		s.cloneHeartbeatCancel = nil
	}
	if s.peer == nil {
		return fmt.Errorf("sync peer is not paired")
	}
	trusted, err := s.peerIsTrusted()
	if err != nil {
		return err
	}
	if !trusted {
		return fmt.Errorf("sync peer is not paired")
	}
	if s.peer.address == "" || Fingerprint(s.peer.public) != peer.Fingerprint {
		return fmt.Errorf("sync peer endpoint is unavailable")
	}
	return s.peer.syncComplete(ctx, successful, s.clone)
}

// NotifySyncCancelled tells a trusted peer that the user cancelled Sync before completion.
func (s *Session) NotifySyncCancelled(ctx context.Context, peer Peer) error {
	s.stopSyncHeartbeat()
	if s.clone {
		return fmt.Errorf("clone sessions cannot report Sync cancellation")
	}
	if s.peer == nil {
		return fmt.Errorf("sync peer is not paired")
	}
	trusted, err := s.peerIsTrusted()
	if err != nil {
		return err
	}
	if !trusted {
		return fmt.Errorf("sync peer is not paired")
	}
	if s.peer.address == "" || Fingerprint(s.peer.public) != peer.Fingerprint {
		return fmt.Errorf("sync peer endpoint is unavailable")
	}
	return s.peer.syncCancelled(ctx, s.clone)
}

func (s *Session) startSyncHeartbeat(remote *peer) {
	if remote == nil || s.syncHeartbeatCancel != nil || s.clone {
		return
	}
	ctx, cancel := context.WithCancel(s.ctx)
	s.syncHeartbeatCancel = cancel
	go func() {
		ticker := time.NewTicker(syncHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				response, err := remote.client.postPairing(ctx, remote.address, syncHeartbeatPath, struct{}{}, remote.public, handshakeLimit)
				if err == nil {
					_ = response.Body.Close()
				}
			}
		}
	}()
}

func (s *Session) stopSyncHeartbeat() {
	if s.syncHeartbeatCancel != nil {
		s.syncHeartbeatCancel()
		s.syncHeartbeatCancel = nil
	}
}

func (s *Session) startCloneHeartbeat(remote *peer) {
	ctx, cancel := context.WithCancel(s.ctx)
	s.cloneHeartbeatCancel = cancel
	go func() {
		ticker := time.NewTicker(cloneHeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				response, err := remote.client.postPairing(ctx, remote.address, cloneHeartbeatPath, struct{}{}, remote.public, handshakeLimit)
				if err == nil {
					_ = response.Body.Close()
				}
			}
		}
	}()
}

// Connect performs Hello and checks the advertised fingerprint without establishing trust.
func (s *Session) Connect(ctx context.Context, candidate Peer) (*mldsa.PublicKey, error) {
	logger := slog.With("component", "sync.connection")
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if candidate.Port < 1 || candidate.Port > 65535 {
		return nil, fmt.Errorf("invalid sync endpoint")
	}
	if candidate.Version != protocolVersion {
		return nil, fmt.Errorf("invalid sync endpoint")
	}
	if candidate.Fingerprint == "" {
		return nil, fmt.Errorf("invalid sync endpoint")
	}
	addresses := candidate.IPs
	if len(addresses) == 0 && candidate.IP != nil {
		addresses = []net.IP{candidate.IP}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("sync endpoint has no usable addresses")
	}
	var lastErr error
	for _, address := range addresses {
		if address == nil {
			continue
		}
		if address.IsUnspecified() || address.IsMulticast() {
			continue
		}
		requestCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		stop := context.AfterFunc(s.ctx, cancel)
		hostPort := net.JoinHostPort(address.String(), strconv.Itoa(candidate.Port))
		logger.Debug("connecting to Sync peer", "device", candidate.Name, "endpoint", hostPort)
		public, vaultRef, err := s.client.HelloWithVault(requestCtx, hostPort)
		stop()
		cancel()
		if err != nil {
			failureKind := "connection"
			var certificateError *tls.CertificateVerificationError
			if errors.As(err, &certificateError) {
				failureKind = "tls_authentication"
			}
			logger.Debug("Sync peer connection failed", "device", candidate.Name, "endpoint", hostPort, "failure_kind", failureKind, "timed_out", errors.Is(err, context.DeadlineExceeded), "error", err)
			lastErr = err
			continue
		}
		if Fingerprint(public) != candidate.Fingerprint {
			logger.Warn("Sync peer identity did not match discovery", "device", candidate.Name, "endpoint", hostPort)
			return nil, fmt.Errorf("sync peer fingerprint does not match discovery")
		}
		s.peer = &peer{client: s.client, address: hostPort, public: public}
		s.peerVault = vaultRef
		logger.Debug("selected Sync peer endpoint", "device", candidate.Name, "endpoint", hostPort)
		return public, nil
	}
	if err := s.ctx.Err(); err != nil {
		return nil, err
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("sync endpoint has no usable addresses")
}

// VaultReferences returns display information for this vault and the authenticated peer.
func (s *Session) VaultReferences() (VaultReference, VaultReference) {
	return s.vaultRef, s.peerVault
}

// RemoteVaultInfo retrieves a paired peer's vault reference.
func (s *Session) RemoteVaultInfo(ctx context.Context) (VaultReference, error) {
	if s.peer == nil || s.peer.address == "" {
		return VaultReference{}, fmt.Errorf("sync peer endpoint is unavailable")
	}
	trusted, err := s.peerIsTrusted()
	if err != nil {
		return VaultReference{}, err
	}
	if !trusted {
		return VaultReference{}, fmt.Errorf("sync peer is not paired")
	}
	info, err := s.peer.VaultInfo(ctx)
	if err != nil {
		return VaultReference{}, err
	}
	s.peerVault = info
	return info, nil
}

func (s *Session) peerIsTrusted() (bool, error) {
	if s.clone && s.clonePeer != nil && s.peer != nil {
		return bytes.Equal(s.clonePeer.PublicKey, s.peer.public.Bytes()), nil
	}
	if s.vault == nil {
		return s.clonePeer != nil && s.peer != nil &&
			bytes.Equal(s.clonePeer.PublicKey, s.peer.public.Bytes()), nil
	}
	peers, err := s.store.ListSyncPeers(s.vault)
	if err != nil {
		return false, err
	}
	return isTrustedPeer(peers, s.peer.public), nil
}

// ConnectLink resolves a manual link and verifies the HTTPS identity it names.
func (s *Session) ConnectLink(ctx context.Context, link ConnectionLink) (*mldsa.PublicKey, Peer, error) {
	addresses, err := net.DefaultResolver.LookupIP(ctx, "ip", link.Host)
	if err != nil {
		return nil, Peer{}, fmt.Errorf("resolve Sync link host: %w", err)
	}
	peer := Peer{IPs: addresses, Port: link.Port, Fingerprint: link.Fingerprint, Version: protocolVersion, Name: link.Host}
	public, err := s.Connect(ctx, peer)
	return public, peer, err
}

// Stop closes the active session and releases its network resources.
func (m *Manager) Stop() error {
	m.mu.Lock()
	s := m.session
	m.mu.Unlock()
	return m.stop(s)
}

func (m *Manager) stop(s *Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s == nil || m.session != s {
		return nil
	}
	s.cancel()
	slog.Debug("stopping Sync session", "component", "sync.session", "device", s.deviceName)
	var err error
	if s.discovery != nil {
		err = s.discovery.Stop()
	}
	s.client.CloseIdleConnections()
	serverErr := <-s.done
	if s.pairing != nil {
		s.pairing.clear()
	}
	m.session = nil
	if errors.Is(serverErr, context.Canceled) {
		serverErr = nil
	}
	s.stopErr = errors.Join(err, serverErr)
	if s.stopErr != nil {
		slog.Warn("Sync session stopped with an error", "component", "sync.session", "error", s.stopErr)
	}
	s.stopped.Done()
	return s.stopErr
}
