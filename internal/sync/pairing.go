// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/mldsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

const pairingTimeout = 2 * time.Minute

const syncCompletePath = "/sync/session/complete"
const cloneHeartbeatPath = "/sync/clone/heartbeat"
const cloneHeartbeatInterval = 10 * time.Second
const cloneHeartbeatTimeout = 45 * time.Second
const syncHeartbeatInterval = 10 * time.Second
const syncHeartbeatTimeout = 45 * time.Second

var (
	errPairingCancelled   = errors.New("pairing cancelled")
	errPairingCompleted   = errors.New("pairing completed. Remove the paired device to disconnect it")
	errPairingInterrupted = errors.New(
		"pairing was interrupted. Remove any partial pairing on both devices and try again",
	)
	errPairingTimeout = errors.New("pairing timed out. Start again on both devices")
)

func isPairingCancellation(err error) bool {
	var responseErr *pairingHTTPError
	return errors.As(err, &responseErr) && (responseErr.status == http.StatusForbidden ||
		responseErr.status == http.StatusGone || responseErr.status == http.StatusNotFound)
}

type pairingRequest struct {
	Name  string         `json:"name"`
	Nonce string         `json:"nonce"`
	Vault VaultReference `json:"vault"`
	Clone bool           `json:"clone,omitempty"`
}

type pairingResponse struct {
	ID    string         `json:"id"`
	Code  string         `json:"code"`
	Name  string         `json:"name"`
	Vault VaultReference `json:"vault"`
}

type pairingAction struct {
	ID   string `json:"id"`
	Code string `json:"code,omitempty"`
}

type pairingState struct {
	name      string
	public    *mldsa.PublicKey
	clone     bool
	code      string
	answered  bool
	writing   bool
	persisted bool
	decision  chan bool
	done      chan struct{}
	finished  bool
	result    error
}

type pairingServer struct {
	mu               sync.Mutex
	store            pairingTrustStore
	vault            *model.Vault
	name             string
	vaultRef         VaultReference
	public           *mldsa.PublicKey
	pending          map[string]*pairingState
	incoming         chan PairingRequest
	syncRequests     chan string
	syncOutcomes     chan bool
	syncLost         chan struct{}
	syncProgress     chan SyncProgressEvent
	cloneLost        chan struct{}
	cloneDeadline    time.Time
	cloneTimer       *time.Timer
	syncDeadline     time.Time
	syncTimer        *time.Timer
	activePeer       *mldsa.PublicKey
	activeClone      bool
	outcomeSent      bool
	outcomeCancelled bool
}

// SyncProgressEvent reports an item mutation received from a trusted peer.
type SyncProgressEvent struct {
	DeviceName string
	ItemID     string
	Title      string
	Type       model.ItemType
	Started    bool
	Completed  bool
	Failed     bool
}

type pairingTrustStore interface {
	ListSyncPeers(*model.Vault) ([]storage.SyncPeer, error)
	AddSyncPeer(*model.Vault, storage.SyncPeer) error
	RemoveSyncPeer(*model.Vault, []byte) error
	LoadItem(*model.Vault, string) (*model.Item, error)
	StoreItem(*model.Vault, *model.Item) error
	DeleteItem(*model.Vault, string) error
	VerifyAttachment(*model.Vault, string, cxf.File) (bool, error)
	WriteAttachment(*model.Vault, string, cxf.File, io.Writer) error
	ReceiveAttachment(*model.Vault, string, cxf.File, io.Reader) (bool, error)
	CleanupAttachments(*model.Vault, string, []string) error
}

// PairingRequest is a pairing request received by this instance.
type PairingRequest struct {
	ID    string
	Name  string
	Vault VaultReference
	peer  *pairingServer
	state *pairingState
}

// Accept approves the displayed code and releases the waiting planner.
func (r PairingRequest) Accept() error {
	return r.peer.decide(r.ID, true)
}

// Decline declines the request without adding any trusted device.
func (r PairingRequest) Decline() error {
	return r.peer.decide(r.ID, false)
}

// Cancel immediately clears this device's pending request.
func (r PairingRequest) Cancel() error {
	r.peer.mu.Lock()
	defer r.peer.mu.Unlock()
	if r.state.finished && r.state.result == nil {
		return errPairingCompleted
	}
	if r.peer.pending[r.ID] == r.state {
		delete(r.peer.pending, r.ID)
		r.state.finish(errPairingCancelled)
	}
	return nil
}

// VerificationCode returns the TLS-bound code for this request.
func (r PairingRequest) VerificationCode() (string, error) {
	r.peer.mu.Lock()
	defer r.peer.mu.Unlock()
	state := r.peer.pending[r.ID]
	if state == nil {
		return "", errors.New("pairing request is no longer active")
	}
	return state.code, nil
}

// Wait returns when both devices have acknowledged storing trust.
func (r PairingRequest) Wait(ctx context.Context) error {
	select {
	case <-r.state.done:
		return r.state.result
	case <-ctx.Done():
		return ctx.Err()
	}
}

func newPairingServer(
	store *storage.FileStorage,
	vault *model.Vault,
	name string,
	public *mldsa.PublicKey,
) *pairingServer {
	return &pairingServer{
		store:        store,
		vault:        vault,
		name:         name,
		public:       public,
		pending:      make(map[string]*pairingState),
		incoming:     make(chan PairingRequest, 1),
		syncRequests: make(chan string, 1),
		syncOutcomes: make(chan bool, 1),
		syncLost:     make(chan struct{}, 1),
		syncProgress: make(chan SyncProgressEvent, 2*maxManifestItems),
		cloneLost:    make(chan struct{}, 1),
	}
}

func (p *pairingServer) reportSyncProgress(event SyncProgressEvent) {
	select {
	case p.syncProgress <- event:
	default:
	}
}

func (p *pairingServer) request(w http.ResponseWriter, r *http.Request) {
	logger := slog.With("component", "sync.pairing")
	request, err := readPairingRequest(r.Body)
	if err != nil {
		http.Error(w, "invalid pairing request", http.StatusBadRequest)
		return
	}
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		logger.Debug("pairing peer authentication failed", "error", err)
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	if _, err := p.store.ListSyncPeers(p.vault); err != nil {
		http.Error(w, "could not read trusted devices", http.StatusInternalServerError)
		return
	}
	var nonce [32]byte
	decoded, err := hex.Decode(nonce[:], []byte(request.Nonce))
	if err != nil {
		http.Error(w, "invalid pairing nonce", http.StatusBadRequest)
		return
	}
	if decoded != len(nonce) {
		http.Error(w, "invalid pairing nonce", http.StatusBadRequest)
		return
	}
	key, err := pairingCode(r.TLS, nonce[:], remote.Bytes(), p.public.Bytes())
	if err != nil {
		http.Error(w, "could not bind pairing verification", http.StatusBadRequest)
		return
	}
	var idBytes [24]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		http.Error(w, "could not create pairing session", http.StatusInternalServerError)
		return
	}
	id := hex.EncodeToString(idBytes[:])
	state := &pairingState{
		name:     request.Name,
		public:   remote,
		clone:    request.Clone,
		code:     key,
		decision: make(chan bool, 1),
		done:     make(chan struct{}),
	}
	logger.Debug("pairing request received", "device", request.Name)
	p.mu.Lock()
	if p.activePeer != nil || len(p.pending) > 0 {
		p.mu.Unlock()
		http.Error(w, "another Sync peer is already active or awaiting approval", http.StatusConflict)
		return
	}
	p.pending[id] = state
	p.mu.Unlock()
	keep := false
	defer func() {
		if keep {
			return
		}
		p.mu.Lock()
		if p.pending[id] == state {
			delete(p.pending, id)
		}
		state.finish(errors.New("pairing ended before confirmation"))
		p.mu.Unlock()
	}()
	select {
	case p.incoming <- PairingRequest{
		ID:    id,
		Name:  request.Name,
		Vault: request.Vault,
		peer:  p,
		state: state,
	}:
	case <-r.Context().Done():
		logger.Debug("pairing request canceled", "device", request.Name)
		return
	case <-time.After(pairingTimeout):
		logger.Debug("pairing request timed out", "device", request.Name)
		http.Error(w, "pairing request timed out", http.StatusRequestTimeout)
		return
	}
	keep = true
	writePairingJSON(w, pairingResponse{
		ID:   id,
		Code: key,
		Name: p.name,
	})
	time.AfterFunc(pairingTimeout, func() {
		p.mu.Lock()
		if p.pending[id] == state {
			delete(p.pending, id)
		}
		state.finish(errPairingTimeout)
		p.mu.Unlock()
	})
}

func (s *pairingState) finish(err error) {
	if s.finished {
		return
	}
	s.finished = true
	s.result = err
	close(s.done)
}

// releaseConfirmation lets the peer retry when confirmation stops before the
// trust write is admitted.
func (p *pairingServer) releaseConfirmation(id string, state *pairingState) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.pending[id] == state && !state.finished {
		state.writing = false
	}
}

func (p *pairingServer) complete(w http.ResponseWriter, r *http.Request) {
	logger := slog.With("component", "sync.pairing")
	var action pairingAction
	if err := json.UnmarshalRead(io.LimitReader(r.Body, 4096), &action); err != nil {
		http.Error(w, "invalid pairing completion", http.StatusBadRequest)
		return
	}
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	p.mu.Lock()
	state := p.pending[action.ID]
	if state == nil {
		p.mu.Unlock()
		http.Error(w, "pairing session is unavailable", http.StatusNotFound)
		return
	}
	if !state.public.Equal(remote) || !state.persisted {
		p.mu.Unlock()
		http.Error(w, "pairing confirmation is required", http.StatusForbidden)
		return
	}
	if action.Code != state.code {
		p.mu.Unlock()
		http.Error(w, "verification code does not match", http.StatusForbidden)
		return
	}
	allowed, started := p.beginSyncLocked(remote)
	if !allowed {
		p.mu.Unlock()
		http.Error(w, "another Sync peer is already active", http.StatusConflict)
		return
	}
	p.activeClone = state.clone
	if state.clone {
		p.cloneDeadline = time.Now().Add(cloneHeartbeatTimeout)
		p.cloneTimer = time.AfterFunc(cloneHeartbeatTimeout, p.expireClone)
	} else {
		p.syncDeadline = time.Now().Add(syncHeartbeatTimeout)
		p.syncTimer = time.AfterFunc(syncHeartbeatTimeout, p.expireSync)
	}
	state.finish(nil)
	delete(p.pending, action.ID)
	p.mu.Unlock()
	if started {
		p.syncRequests <- state.name
	}
	logger.Debug("pairing completed", "device", state.name)
	writePairingJSON(w, struct {
		Stored bool `json:"stored"`
	}{Stored: true})
}

func (p *pairingServer) beginSync(remote *mldsa.PublicKey, name string) bool {
	p.mu.Lock()
	allowed, started := p.beginSyncLocked(remote)
	if started {
		p.syncDeadline = time.Now().Add(syncHeartbeatTimeout)
		p.syncTimer = time.AfterFunc(syncHeartbeatTimeout, p.expireSync)
	}
	p.mu.Unlock()
	if started {
		p.syncRequests <- name
	}
	return allowed
}

// beginSyncLocked admits one trusted peer and reports whether it started a new
// exchange. The caller sends the sync request after releasing the lock.
func (p *pairingServer) beginSyncLocked(remote *mldsa.PublicKey) (allowed, started bool) {
	for _, pending := range p.pending {
		if !pending.public.Equal(remote) {
			return false, false
		}
	}
	if p.activePeer != nil {
		return p.activePeer.Equal(remote), false
	}
	p.activePeer = remote
	return true, true
}

func trustedPeerName(peers []storage.SyncPeer, remote *mldsa.PublicKey) string {
	for _, peer := range peers {
		if bytes.Equal(peer.PublicKey, remote.Bytes()) {
			if peer.Name != "" {
				return peer.Name
			}
			break
		}
	}
	return "Trusted device"
}

func (p *pairingServer) completeSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var completion struct {
		Successful bool `json:"successful"`
		Clone      bool `json:"clone,omitempty"`
		Cancelled  bool `json:"cancelled,omitempty"`
	}
	if err := json.UnmarshalRead(io.LimitReader(r.Body, 1024), &completion); err != nil {
		http.Error(w, "invalid sync completion", http.StatusBadRequest)
		return
	}
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	trusted, err := p.store.ListSyncPeers(p.vault)
	if err != nil {
		http.Error(w, "could not read trusted devices", http.StatusInternalServerError)
		return
	}
	if !isTrustedPeer(trusted, remote) {
		http.Error(w, "sync peer is not paired", http.StatusForbidden)
		return
	}
	p.mu.Lock()
	active := p.activePeer != nil && p.activePeer.Equal(remote)
	if !active || completion.Clone != p.activeClone {
		p.mu.Unlock()
		http.Error(w, "sync peer is not active", http.StatusForbidden)
		return
	}
	if p.outcomeSent {
		p.mu.Unlock()
		http.Error(w, "sync completion was already received", http.StatusConflict)
		return
	}
	if completion.Cancelled && (completion.Successful || completion.Clone) {
		p.mu.Unlock()
		http.Error(w, "invalid sync cancellation outcome", http.StatusBadRequest)
		return
	}
	if completion.Clone && !completion.Successful {
		if err := p.store.RemoveSyncPeer(p.vault, remote.Bytes()); err != nil {
			p.mu.Unlock()
			http.Error(w, "could not update Clone pairing", http.StatusInternalServerError)
			return
		}
	}
	p.outcomeSent = true
	p.outcomeCancelled = completion.Cancelled
	if p.cloneTimer != nil {
		p.cloneTimer.Stop()
	}
	if p.syncTimer != nil {
		p.syncTimer.Stop()
	}
	p.mu.Unlock()
	p.syncOutcomes <- completion.Successful
	w.WriteHeader(http.StatusNoContent)
}

func (p *pairingServer) syncHeartbeat(w http.ResponseWriter, r *http.Request) {
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.activeClone || p.outcomeSent || p.activePeer == nil || !p.activePeer.Equal(remote) {
		http.Error(w, "sync session is not active", http.StatusForbidden)
		return
	}
	p.syncDeadline = time.Now().Add(syncHeartbeatTimeout)
	if p.syncTimer != nil {
		p.syncTimer.Stop()
	}
	p.syncTimer = time.AfterFunc(syncHeartbeatTimeout, p.expireSync)
	w.WriteHeader(http.StatusNoContent)
}

func (p *pairingServer) expireSync() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.activeClone || p.outcomeSent || p.activePeer == nil {
		return
	}
	if remaining := time.Until(p.syncDeadline); remaining > 0 {
		p.syncTimer = time.AfterFunc(remaining, p.expireSync)
		return
	}
	p.outcomeSent = true
	select {
	case p.syncLost <- struct{}{}:
	default:
	}
}

func (p *pairingServer) cloneHeartbeat(w http.ResponseWriter, r *http.Request) {
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.activeClone || p.outcomeSent || p.activePeer == nil || !p.activePeer.Equal(remote) {
		http.Error(w, "clone session is not active", http.StatusForbidden)
		return
	}
	p.cloneDeadline = time.Now().Add(cloneHeartbeatTimeout)
	w.WriteHeader(http.StatusNoContent)
}

func (p *pairingServer) expireClone() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.activeClone || p.outcomeSent {
		return
	}
	if remaining := time.Until(p.cloneDeadline); remaining > 0 {
		p.cloneTimer = time.AfterFunc(remaining, p.expireClone)
		return
	}
	p.outcomeSent = true
	select {
	case p.cloneLost <- struct{}{}:
	default:
	}
}

func readPairingRequest(body io.Reader) (pairingRequest, error) {
	var request pairingRequest
	err := json.UnmarshalRead(io.LimitReader(body, 4096), &request)
	if err != nil {
		return pairingRequest{}, err
	}
	if request.Name == "" {
		return pairingRequest{}, errors.New("device name is required")
	}
	if request.Clone && request.Vault.ID != "" {
		return pairingRequest{}, errors.New("clone pairing must not include a local vault")
	}
	if len(request.Name) > 255 {
		return pairingRequest{}, errors.New("device name is too long")
	}
	if strings.TrimSpace(request.Name) != request.Name {
		return pairingRequest{}, errors.New("device name has surrounding whitespace")
	}
	return request, nil
}

func (p *pairingServer) confirm(w http.ResponseWriter, r *http.Request) {
	var action pairingAction
	if err := json.UnmarshalRead(io.LimitReader(r.Body, 4096), &action); err != nil {
		http.Error(w, "invalid pairing confirmation", http.StatusBadRequest)
		return
	}
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	p.mu.Lock()
	state := p.pending[action.ID]
	if state == nil {
		p.mu.Unlock()
		http.Error(w, "pairing session is unavailable", http.StatusNotFound)
		return
	}
	if !state.public.Equal(remote) {
		p.mu.Unlock()
		http.Error(w, "pairing session is unavailable", http.StatusNotFound)
		return
	}
	if action.Code != state.code {
		p.mu.Unlock()
		http.Error(w, "verification code does not match", http.StatusBadRequest)
		return
	}
	if state.writing || state.persisted {
		p.mu.Unlock()
		http.Error(w, "pairing confirmation has already started", http.StatusConflict)
		return
	}
	state.writing = true
	p.mu.Unlock()
	select {
	case accepted := <-state.decision:
		if !accepted {
			http.Error(w, "pairing declined", http.StatusForbidden)
			return
		}
	case <-state.done:
		http.Error(w, "pairing canceled; start again on both devices", http.StatusGone)
		return
	case <-r.Context().Done():
		p.releaseConfirmation(action.ID, state)
		return
	case <-time.After(pairingTimeout):
		p.releaseConfirmation(action.ID, state)
		http.Error(w, "pairing request timed out", http.StatusRequestTimeout)
		return
	}
	// Cancellation and trust-write admission are ordered by the same lock.
	p.mu.Lock()
	if p.pending[action.ID] != state || state.finished || r.Context().Err() != nil {
		p.mu.Unlock()
		http.Error(w, "pairing canceled; start again on both devices", http.StatusGone)
		return
	}
	p.mu.Unlock()
	if err := p.store.AddSyncPeer(p.vault, storage.SyncPeer{
		PublicKey: remote.Bytes(),
		Name:      state.name,
	}); err != nil {
		p.mu.Lock()
		if p.pending[action.ID] == state {
			delete(p.pending, action.ID)
			state.finish(errPairingInterrupted)
		}
		p.mu.Unlock()
		http.Error(w, errPairingInterrupted.Error(), http.StatusInternalServerError)
		return
	}
	p.mu.Lock()
	if p.pending[action.ID] != state {
		p.mu.Unlock()
		http.Error(w, errPairingCancelled.Error(), http.StatusGone)
		return
	}
	state.persisted = true
	p.mu.Unlock()
	writePairingJSON(w, struct {
		Stored bool `json:"stored"`
	}{Stored: true})
}

func (p *pairingServer) cancel(w http.ResponseWriter, r *http.Request) {
	var action pairingAction
	if err := json.UnmarshalRead(io.LimitReader(r.Body, 4096), &action); err != nil {
		http.Error(w, "invalid pairing cancellation", http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	state := p.pending[action.ID]
	if state == nil {
		p.mu.Unlock()
		http.Error(w, "pairing session is unavailable", http.StatusNotFound)
		return
	}
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		p.mu.Unlock()
		http.Error(w, "pairing session is unavailable", http.StatusNotFound)
		return
	}
	if !state.public.Equal(remote) {
		p.mu.Unlock()
		http.Error(w, "pairing session is unavailable", http.StatusNotFound)
		return
	}
	delete(p.pending, action.ID)
	state.finish(errPairingCancelled)
	p.mu.Unlock()
	w.WriteHeader(http.StatusOK)
}

func (p *pairingServer) decide(id string, accept bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	state := p.pending[id]
	if state == nil {
		return errors.New("pairing request is no longer active")
	}
	if state.answered {
		return errors.New("pairing request has already been answered")
	}
	select {
	case state.decision <- accept:
		state.answered = true
		if accept {
			slog.Debug("pairing request accepted", "component", "sync.pairing", "device", state.name)
		} else {
			delete(p.pending, id)
			state.finish(errPairingCancelled)
			slog.Debug("pairing request declined", "component", "sync.pairing", "device", state.name)
		}
		return nil
	default:
		return errors.New("pairing request has already been answered")
	}
}

func (p *pairingServer) clear() {
	p.mu.Lock()
	if p.cloneTimer != nil {
		p.cloneTimer.Stop()
	}
	if p.syncTimer != nil {
		p.syncTimer.Stop()
	}
	p.outcomeSent = true
	for _, state := range p.pending {
		state.code = ""
		state.finish(errors.New("sync session stopped"))
	}
	clear(p.pending)
	p.mu.Unlock()
}

func pairingCode(state *tls.ConnectionState, nonce, plannerKey, responderKey []byte) (string, error) {
	if state == nil {
		return "", errors.New("peer connection state is unavailable")
	}
	exporter, err := state.ExportKeyingMaterial("EXPORTER-Paw-Sync-Pairing", nonce, 32)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, exporter)
	_, _ = mac.Write(nonce)
	_, _ = mac.Write(plannerKey)
	_, _ = mac.Write(responderKey)
	sum := mac.Sum(nil)
	value := (uint32(sum[0])<<24 | uint32(sum[1])<<16 | uint32(sum[2])<<8 | uint32(sum[3])) % 1_000_000
	clear(exporter)
	clear(sum)
	return fmt.Sprintf("%06d", int(value)), nil
}

func writePairingJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(w, value, json.Deterministic(true)); err != nil {
		http.Error(w, "could not encode pairing response", http.StatusInternalServerError)
	}
}

// Pairing represents a planner-side request awaiting responder approval.
type Pairing struct {
	session       *Session
	peer          *peer
	id            string
	code          string
	name          string
	vault         VaultReference
	public        *mldsa.PublicKey
	mu            sync.Mutex
	confirmCancel context.CancelFunc
	result        error
}

// Code is shown to the user for comparison with the responder's code.
func (p *Pairing) Code() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.code
}

// VaultReference returns the authenticated peer's public vault display information.
func (p *Pairing) VaultReference() VaultReference { return p.vault }

// Confirm persists trust locally only after the responder acknowledged its own persistence.
func (p *Pairing) Confirm(ctx context.Context, code string) (result error) {
	logger := slog.With("component", "sync.pairing", "device", p.name)
	logger.Debug("confirming Sync pairing")
	p.mu.Lock()
	if p.result != nil {
		result := p.result
		p.mu.Unlock()
		return result
	}
	if p.confirmCancel != nil {
		p.mu.Unlock()
		return errors.New("pairing confirmation is in progress")
	}
	if code != p.code {
		p.mu.Unlock()
		return errors.New("verification code does not match")
	}
	if p.session.vault != nil && p.session.vault.Key() == nil {
		p.mu.Unlock()
		return errors.New("unlock the vault before confirming pairing")
	}
	requestCtx, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(p.session.ctx, cancel)
	p.confirmCancel = cancel
	public := p.public
	remotePeer := p.peer
	p.mu.Unlock()
	defer func() {
		stop()
		cancel()
		p.mu.Lock()
		defer p.mu.Unlock()
		p.confirmCancel = nil
		if p.result == errPairingCancelled {
			result = p.result
			return
		}
		p.result = result
		if result == nil {
			p.result = errPairingCompleted
			p.code = ""
			p.public = nil
			p.peer = nil
		}
	}()
	action := pairingAction{
		ID:   p.id,
		Code: code,
	}
	if !p.canContinue(requestCtx) {
		return errPairingInterrupted
	}
	if err := remotePeer.pairingAction(requestCtx, pairConfirmPath, action); err != nil {
		logger.Warn("Sync pairing confirmation failed", "error", err)
		if isPairingCancellation(err) {
			return errPairingCancelled
		}
		return errPairingInterrupted
	}
	// Admit the atomic write under the state lock, then let it finish independently.
	if !p.canContinue(requestCtx) {
		return errPairingInterrupted
	}
	peer := storage.SyncPeer{
		PublicKey: public.Bytes(),
		Name:      p.name,
	}
	if p.session.vault != nil {
		if err := p.session.store.AddSyncPeer(p.session.vault, peer); err != nil {
			return errPairingInterrupted
		}
	}
	if !p.canContinue(requestCtx) {
		return errPairingInterrupted
	}
	if err := remotePeer.pairingAction(requestCtx, pairCompletePath, action); err != nil {
		logger.Warn("Sync pairing completion failed", "error", err)
		return errPairingInterrupted
	}
	if p.session.vault == nil {
		p.session.clonePeer = &peer
		p.session.startCloneHeartbeat(remotePeer)
	} else {
		p.session.startSyncHeartbeat(remotePeer)
	}
	logger.Debug("Sync pairing completed")
	return nil
}

func (p *Pairing) canContinue(ctx context.Context) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.result == nil && ctx.Err() == nil && p.session.ctx.Err() == nil
}

// Cancel clears local state and attempts to notify the responder.
func (p *Pairing) Cancel(ctx context.Context) error {
	slog.Debug("canceling Sync pairing", "component", "sync.pairing", "device", p.name)
	p.mu.Lock()
	if p.result == errPairingCompleted {
		p.mu.Unlock()
		return errPairingCompleted
	}
	if p.result == errPairingCancelled {
		p.mu.Unlock()
		return nil
	}
	remotePeer := p.peer
	p.result = errPairingCancelled
	if p.confirmCancel != nil {
		p.confirmCancel()
	}
	p.code = ""
	p.public = nil
	p.peer = nil
	p.mu.Unlock()
	if ctx.Err() == nil {
		go func() {
			requestCtx, cancel := context.WithTimeout(p.session.ctx, handshakeLimit)
			defer cancel()
			if err := remotePeer.pairingAction(requestCtx, pairCancelPath, pairingAction{ID: p.id}); err != nil {
				slog.Debug("could not notify pairing cancel", "component", "sync.pairing", "error", err)
			}
		}()
	}
	return nil
}

// PairingRequests reports requests awaiting a local Accept or Decline decision.
func (s *Session) PairingRequests() <-chan PairingRequest {
	return s.pairing.incoming
}

// IncomingSyncs reports the trusted device that started a Sync session.
func (s *Session) IncomingSyncs() <-chan string {
	return s.pairing.syncRequests
}

// SyncOutcomes reports the remote planner's completion result.
func (s *Session) SyncOutcomes() <-chan bool {
	return s.pairing.syncOutcomes
}

// SyncConnectionLost reports when an active Sync peer stops sending authenticated heartbeats.
func (s *Session) SyncConnectionLost() <-chan struct{} { return s.pairing.syncLost }

// SyncOutcomeCancelled reports whether the received Sync outcome was an explicit cancellation.
func (s *Session) SyncOutcomeCancelled() bool {
	s.pairing.mu.Lock()
	defer s.pairing.mu.Unlock()
	return s.pairing.outcomeCancelled
}

// CloneConnectionLost reports an unknown Clone outcome after the peer stops responding.
func (s *Session) CloneConnectionLost() <-chan struct{} { return s.pairing.cloneLost }

// SyncProgress reports item mutations received from a trusted peer.
func (s *Session) SyncProgress() <-chan SyncProgressEvent {
	return s.pairing.syncProgress
}

func (s *Session) requestPairing(ctx context.Context, candidate Peer) (*Pairing, error) {
	logger := slog.With("component", "sync.pairing", "device", candidate.Name)
	logger.Debug("requesting Sync pairing")
	remote, err := s.Connect(ctx, candidate)
	if err != nil {
		return nil, err
	}
	var known []storage.SyncPeer
	if s.vault != nil {
		known, err = s.store.ListSyncPeers(s.vault)
		if err != nil {
			return nil, err
		}
	}
	for _, existing := range known {
		if bytes.Equal(existing.PublicKey, remote.Bytes()) {
			return nil, errors.New("device is already paired. Remove it locally before pairing again")
		}
	}
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	request := pairingRequest{
		Name:  s.deviceName,
		Nonce: hex.EncodeToString(nonce[:]),
		Vault: s.vaultRef,
		Clone: s.clone,
	}
	pairingPeer := &peer{client: s.client, address: s.peer.address, public: remote}
	response, err := pairingPeer.pair(ctx, request, s.public)
	if err != nil {
		logger.Debug("Sync pairing request failed", "error", err)
		return nil, err
	}
	logger.Debug("Sync pairing request accepted")
	return &Pairing{
		session: s,
		peer:    pairingPeer,
		id:      response.ID,
		code:    response.Code,
		name:    response.Name,
		vault:   response.Vault,
		public:  remote,
	}, nil
}

// PairedPeers lists identities trusted by the active vault.
func (s *Session) PairedPeers() ([]storage.SyncPeer, error) {
	return s.store.ListSyncPeers(s.vault)
}

// TrustedFingerprints returns fingerprints for the peers trusted by the active vault.
func (s *Session) TrustedFingerprints() (map[string]bool, error) {
	peers, err := s.PairedPeers()
	if err != nil {
		return nil, err
	}
	fingerprints := make(map[string]bool, len(peers))
	for _, peer := range peers {
		public, err := mldsa.NewPublicKey(mldsa.MLDSA44(), peer.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parse trusted Sync device key: %w", err)
		}
		fingerprints[Fingerprint(public)] = true
	}
	return fingerprints, nil
}

// RemovePairedPeer removes a peer from the active vault's trust store.
func (s *Session) RemovePairedPeer(publicKey []byte) error {
	return s.store.RemoveSyncPeer(s.vault, publicKey)
}

// ConnectTrusted authenticates the TLS peer and requires its key to be paired.
func (s *Session) ConnectTrusted(ctx context.Context, peer Peer) (*mldsa.PublicKey, error) {
	public, err := s.Connect(ctx, peer)
	if err != nil {
		return nil, err
	}
	trusted, err := s.peerIsTrusted()
	if err != nil {
		return nil, err
	}
	if trusted {
		slog.Debug("Sync peer is trusted", "component", "sync.connection", "device", peer.Name)
		s.startSyncHeartbeat(s.peer)
		return public, nil
	}
	slog.Debug("Sync peer is not trusted", "component", "sync.connection", "device", peer.Name)
	return nil, errors.New("sync peer is not paired")
}
