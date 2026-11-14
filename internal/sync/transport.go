// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	pairingResponseLimit = 4096
	helloPath            = "/sync/hello"
	manifestPath         = "/sync/manifest"
	itemPath             = "/sync/item"
	attachmentPath       = "/sync/attachment"
	pairRequestPath      = "/sync/pair/request"
	pairConfirmPath      = "/sync/pair/confirm"
	pairCompletePath     = "/sync/pair/complete"
	pairCancelPath       = "/sync/pair/cancel"
	protocolVersion      = "paw-sync/1"
	handshakeLimit       = 5 * time.Second
	syncHeartbeatPath    = "/sync/session/heartbeat"
)

// syncURL builds the HTTPS URL of one Sync endpoint on a peer.
func syncURL(address, path string) string {
	return "https://" + address + path
}

type pairingHTTPError struct {
	status  int
	message string
}

func (e *pairingHTTPError) Error() string {
	return fmt.Sprintf("sync pairing request returned HTTP %d: %s", e.status, e.message)
}

// Server serves the non-sensitive Sync bootstrap endpoint over mutually authenticated TLS.
type Server struct {
	http            *http.Server
	shutdownTimeout time.Duration
}

// VaultReference is the public display information exchanged over Sync HTTPS.
type VaultReference struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Items       int    `json:"items,omitempty"`
	Collections int    `json:"collections,omitempty"`
	Attachments int    `json:"attachments,omitempty"`
}

type helloResponse struct {
	Version string         `json:"version"`
	Vault   VaultReference `json:"vault"`
}

// NewServer creates an HTTP server using the vault's ML-DSA-44 Sync identity.
func NewServer(identity *mldsa.PrivateKey) (*Server, error) {
	return newServer(identity, nil, VaultReference{})
}

func newServer(identity *mldsa.PrivateKey, pairing *pairingServer, vaults ...VaultReference) (*Server, error) {
	var vault VaultReference
	if len(vaults) > 0 {
		vault = vaults[0]
	}
	certificates, err := newSessionCertificates(identity)
	if err != nil {
		return nil, err
	}
	config := &tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			return certificates.current()
		},
		ClientAuth:       tls.RequireAnyClientCert,
		VerifyConnection: verifyConnection,
	}
	mux := http.NewServeMux()
	mux.HandleFunc(helloPath, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != helloPath {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.MarshalWrite(w, helloResponse{Version: protocolVersion, Vault: vault}, json.Deterministic(true))
	})
	if pairing != nil {
		mux.HandleFunc("POST "+pairRequestPath, pairing.request)
		mux.HandleFunc("POST "+pairConfirmPath, pairing.confirm)
		mux.HandleFunc("POST "+pairCompletePath, pairing.complete)
		mux.HandleFunc("POST "+pairCancelPath, pairing.cancel)
		mux.HandleFunc("POST "+syncCompletePath, pairing.completeSync)
		mux.HandleFunc("POST "+syncHeartbeatPath, pairing.syncHeartbeat)
		mux.HandleFunc("POST "+cloneHeartbeatPath, pairing.cloneHeartbeat)
		mux.HandleFunc(manifestPath, pairing.serveManifest)
		mux.HandleFunc(vaultInfoPath, pairing.serveVaultInfo)
		mux.HandleFunc(itemPath, pairing.serveItem)
		mux.HandleFunc(attachmentPath, pairing.serveAttachment)
	}
	return &Server{
		http: &http.Server{
			TLSConfig:         config,
			Handler:           mux,
			ReadHeaderTimeout: handshakeLimit,
		},
		shutdownTimeout: handshakeLimit,
	}, nil
}

// postPairing sends one JSON pairing request pinned to the expected peer.
func (c *Client) postPairing(
	ctx context.Context,
	address, path string,
	payload any,
	expected *mldsa.PublicKey,
	timeout time.Duration,
) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, syncURL(address, path), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	transport := c.pairingTransport(expected)
	transport.ResponseHeaderTimeout = timeout
	defer transport.CloseIdleConnections()
	// Copy the client so this peer's pinned transport and timeout do not mutate the shared one.
	client := *c.http
	client.Transport = transport
	client.Timeout = timeout
	return client.Do(request)
}

func (p *peer) pair(ctx context.Context, request pairingRequest, local *mldsa.PublicKey) (*pairingResponse, error) {
	response, err := p.client.postPairing(ctx, p.address, pairRequestPath, request, p.public, pairingTimeout)
	if err != nil {
		return nil, fmt.Errorf("request Sync pairing: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return nil, fmt.Errorf(
			"sync pairing request returned HTTP %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(message)),
		)
	}
	var result pairingResponse
	limited := io.LimitReader(response.Body, pairingResponseLimit+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read Sync pairing response: %w", err)
	}
	if len(body) > pairingResponseLimit {
		return nil, fmt.Errorf("pairing Sync response exceeds %d bytes", pairingResponseLimit)
	}
	if err := json.UnmarshalRead(bytes.NewReader(body), &result); err != nil {
		return nil, fmt.Errorf("read Sync pairing response: %w", err)
	}
	nonce, err := hex.DecodeString(request.Nonce)
	if err != nil {
		return nil, fmt.Errorf("invalid pairing nonce")
	}
	code, err := pairingCode(response.TLS, nonce, local.Bytes(), p.public.Bytes())
	if err != nil {
		return nil, fmt.Errorf("could not verify responder code: %w", err)
	}
	if code != result.Code {
		return nil, fmt.Errorf("responder verification code does not match TLS session")
	}
	return &result, nil
}

func (p *peer) pairingAction(ctx context.Context, path string, request any) error {
	response, err := p.client.postPairing(ctx, p.address, path, request, p.public, pairingTimeout)
	if err != nil {
		return fmt.Errorf("sync pairing request failed: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return &pairingHTTPError{status: response.StatusCode, message: strings.TrimSpace(string(message))}
	}
	if path == pairConfirmPath || path == pairCompletePath {
		var acknowledgement struct {
			Stored bool `json:"stored"`
		}
		if err := json.UnmarshalRead(io.LimitReader(response.Body, 128), &acknowledgement); err != nil {
			return fmt.Errorf("read responder trust acknowledgement: %w", err)
		}
		if !acknowledgement.Stored {
			return fmt.Errorf("responder did not acknowledge trust persistence")
		}
	}
	return nil
}

func (p *peer) syncComplete(ctx context.Context, successful, clone bool) error {
	return p.syncCompletion(ctx, successful, clone, false)
}

func (p *peer) syncCancelled(ctx context.Context, clone bool) error {
	return p.syncCompletion(ctx, false, clone, true)
}

func (p *peer) syncCompletion(ctx context.Context, successful, clone, cancelled bool) error {
	response, err := p.client.postPairing(ctx, p.address, syncCompletePath, struct {
		Successful bool `json:"successful"`
		Clone      bool `json:"clone,omitempty"`
		Cancelled  bool `json:"cancelled,omitempty"`
	}{
		Successful: successful,
		Clone:      clone,
		Cancelled:  cancelled,
	}, p.public, handshakeLimit)
	if err != nil {
		return fmt.Errorf("send sync completion: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		message, _ := io.ReadAll(io.LimitReader(response.Body, 512))
		return fmt.Errorf("sync completion returned HTTP %d: %s", response.StatusCode, strings.TrimSpace(string(message)))
	}
	return nil
}

func (c *Client) pairingTransport(expected *mldsa.PublicKey) *http.Transport {
	transport := c.http.Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.VerifyConnection = func(state tls.ConnectionState) error {
		if err := verifyConnection(state); err != nil {
			return err
		}
		public, err := peerIdentity(state.PeerCertificates)
		if err != nil {
			return err
		}
		if !public.Equal(expected) {
			return fmt.Errorf("sync peer identity changed during pairing")
		}
		return nil
	}
	return transport
}

// Serve accepts HTTPS connections until the listener closes or the context is canceled.
func (s *Server) Serve(ctx context.Context, listener net.Listener) error {
	if s == nil {
		return fmt.Errorf("sync server is required")
	}
	if s.http == nil {
		return fmt.Errorf("sync server is required")
	}
	if listener == nil {
		return fmt.Errorf("sync server and listener are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	shutdownDone := make(chan error, 1)
	stop := context.AfterFunc(ctx, func() {
		shutdownTimeout := s.shutdownTimeout
		if shutdownTimeout <= 0 {
			shutdownTimeout = handshakeLimit
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err := s.http.Shutdown(shutdownCtx)
		if err != nil {
			err = errors.Join(err, s.http.Close())
		}
		shutdownDone <- err
	})
	err := s.http.Serve(tls.NewListener(listener, s.http.TLSConfig))
	if errors.Is(err, http.ErrServerClosed) {
		err = nil
	}
	if !stop() {
		err = errors.Join(err, <-shutdownDone)
	}
	return err
}

// Client sends the Sync hello request with the local vault identity.
type Client struct {
	http *http.Client
}

// peer holds the authenticated endpoint and identity for one Sync connection.
type peer struct {
	client  *Client
	address string
	public  *mldsa.PublicKey
}

// NewClient creates a client that authenticates with the vault's ML-DSA-44 identity.
func NewClient(identity *mldsa.PrivateKey) (*Client, error) {
	certificates, err := newSessionCertificates(identity)
	if err != nil {
		return nil, err
	}
	transport := &http.Transport{
		TLSClientConfig:       clientTLSConfig(certificates),
		TLSHandshakeTimeout:   handshakeLimit,
		ResponseHeaderTimeout: handshakeLimit,
	}
	return &Client{http: &http.Client{
		Transport: transport,
		Timeout:   2 * handshakeLimit,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}}, nil
}

// Hello authenticates the server certificate and returns its ML-DSA-44 public key.
// Authentication proves key possession only; it does not authorize or trust the peer.
func (c *Client) Hello(ctx context.Context, address string) (*mldsa.PublicKey, error) {
	public, _, err := c.HelloWithVault(ctx, address)
	return public, err
}

// HelloWithVault authenticates the peer and returns its public vault display reference.
func (c *Client) HelloWithVault(ctx context.Context, address string) (*mldsa.PublicKey, VaultReference, error) {
	if c == nil || c.http == nil {
		return nil, VaultReference{}, fmt.Errorf("sync client is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, syncURL(address, helloPath), nil)
	if err != nil {
		return nil, VaultReference{}, fmt.Errorf("create Sync hello request: %w", err)
	}
	response, err := c.http.Do(req)
	if err != nil {
		return nil, VaultReference{}, fmt.Errorf("sync hello request failed: %w", err)
	}
	defer response.Body.Close()
	if response.TLS == nil {
		return nil, VaultReference{}, fmt.Errorf("sync connection did not negotiate required TLS parameters")
	}
	if response.TLS.Version != tls.VersionTLS13 {
		return nil, VaultReference{}, fmt.Errorf("sync connection did not negotiate required TLS parameters")
	}
	if response.TLS.CurveID != tls.X25519MLKEM768 {
		return nil, VaultReference{}, fmt.Errorf("sync connection did not negotiate required TLS parameters")
	}
	if response.StatusCode != http.StatusOK {
		return nil, VaultReference{}, fmt.Errorf("sync hello returned HTTP %d", response.StatusCode)
	}
	var result helloResponse
	if err := json.UnmarshalRead(io.LimitReader(response.Body, 1024), &result); err != nil {
		return nil, VaultReference{}, fmt.Errorf("read Sync hello response: %w", err)
	}
	if result.Version != protocolVersion {
		return nil, VaultReference{}, fmt.Errorf("unsupported sync hello response")
	}
	public, err := peerIdentity(response.TLS.PeerCertificates)
	return public, result.Vault, err
}

// CloseIdleConnections releases connections held by the client transport.
func (c *Client) CloseIdleConnections() {
	if c != nil && c.http != nil {
		c.http.CloseIdleConnections()
	}
}

// AuthenticatedPeerKey returns the peer key from a Sync TLS request. It proves
// key possession but does not authorize or trust the peer.
func AuthenticatedPeerKey(r *http.Request) (*mldsa.PublicKey, error) {
	if r == nil {
		return nil, fmt.Errorf("request is not a sync TLS request")
	}
	if r.TLS == nil {
		return nil, fmt.Errorf("request is not a sync TLS request")
	}
	if err := verifyConnection(*r.TLS); err != nil {
		return nil, err
	}
	return peerIdentity(r.TLS.PeerCertificates)
}

func clientTLSConfig(certificates *sessionCertificates) *tls.Config {
	return &tls.Config{
		MinVersion:       tls.VersionTLS13,
		MaxVersion:       tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{tls.X25519MLKEM768},
		GetClientCertificate: func(*tls.CertificateRequestInfo) (*tls.Certificate, error) {
			return certificates.current()
		},
		InsecureSkipVerify: true, // verifyConnection validates only the presented certificate.
		VerifyConnection:   verifyConnection,
	}
}

func verifyConnection(state tls.ConnectionState) error {
	if state.Version != tls.VersionTLS13 || state.CurveID != tls.X25519MLKEM768 {
		return fmt.Errorf("unsupported sync TLS parameters")
	}
	_, err := peerIdentity(state.PeerCertificates)
	return err
}

func peerIdentity(certificates []*x509.Certificate) (*mldsa.PublicKey, error) {
	if len(certificates) != 1 {
		return nil, fmt.Errorf("sync peer must present one self-signed certificate")
	}
	certificate := certificates[0]
	publicKey, ok := certificate.PublicKey.(*mldsa.PublicKey)
	if !ok || publicKey.Parameters() != mldsa.MLDSA44() {
		return nil, fmt.Errorf("sync peer certificate must use ML-DSA-44")
	}
	if err := certificate.CheckSignatureFrom(certificate); err != nil {
		return nil, fmt.Errorf("invalid sync peer certificate signature")
	}
	if certificate.KeyUsage&x509.KeyUsageDigitalSignature == 0 {
		return nil, fmt.Errorf("sync peer certificate has invalid key usage")
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	for _, usage := range []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth} {
		options := x509.VerifyOptions{
			Roots:     roots,
			KeyUsages: []x509.ExtKeyUsage{usage},
		}
		if _, err := certificate.Verify(options); err != nil {
			return nil, fmt.Errorf("invalid sync peer certificate")
		}
	}
	return publicKey, nil
}

// sessionCertificates keeps one identity while replacing certificates before expiry.
type sessionCertificates struct {
	mu          sync.Mutex
	identity    *mldsa.PrivateKey
	certificate *tls.Certificate
}

// newSessionCertificates generates the initial certificate so startup errors remain immediate.
func newSessionCertificates(identity *mldsa.PrivateKey) (*sessionCertificates, error) {
	certificates := &sessionCertificates{identity: identity}
	if _, err := certificates.current(); err != nil {
		return nil, err
	}
	return certificates, nil
}

// current renews under the lock so concurrent handshakes share one replacement.
func (c *sessionCertificates) current() (*tls.Certificate, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.certificate != nil && time.Until(c.certificate.Leaf.NotAfter) >= 10*time.Minute {
		return c.certificate, nil
	}
	certificate, err := sessionCertificate(c.identity)
	if err != nil {
		return nil, err
	}
	// Replace the pointer; active connections can keep using the previous certificate.
	c.certificate = &certificate
	return c.certificate, nil
}

func sessionCertificate(identity *mldsa.PrivateKey) (tls.Certificate, error) {
	if identity == nil || identity.PublicKey().Parameters() != mldsa.MLDSA44() {
		return tls.Certificate{}, fmt.Errorf("sync requires an ML-DSA-44 identity")
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate Sync certificate serial: %w", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Paw Sync session"},
		NotBefore:             now.Add(-time.Minute),
		NotAfter:              now.Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, identity.PublicKey(), identity)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create Sync session certificate: %w", err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("parse sync session certificate: %w", err)
	}
	return tls.Certificate{
		Certificate: [][]byte{der},
		PrivateKey:  identity,
		Leaf:        leaf,
	}, nil
}
