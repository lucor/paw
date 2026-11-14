// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/mldsa"
	json "encoding/json/v2"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

const (
	// maxSyncItemBytes limits item request and response bodies to 16 MiB.
	maxSyncItemBytes = 16 << 20
	// attachmentBaseTimeout bounds attachment handshakes, headers and small transfers.
	attachmentBaseTimeout = 30 * time.Second
	// attachmentMinRate is the slowest LAN throughput assumed when sizing transfer timeouts.
	attachmentMinRate = 1 << 20
)

type itemRequest struct {
	ID       string      `json:"id"`
	Expected string      `json:"expected"`
	Source   string      `json:"source,omitempty"`
	Item     *model.Item `json:"item,omitempty"`
}

type itemResponse struct {
	Item      *model.Item `json:"item"`
	Committed bool        `json:"committed"`
}

// serveAttachment streams one trusted attachment and verifies it against its metadata.
func (p *pairingServer) serveAttachment(w http.ResponseWriter, r *http.Request) {
	if p.vault.Key() == nil {
		http.Error(w, "vault is locked", http.StatusLocked)
		return
	}
	public, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	peers, err := p.store.ListSyncPeers(p.vault)
	if err != nil || !isTrustedPeer(peers, public) {
		http.Error(w, "sync peer is not paired", http.StatusForbidden)
		return
	}
	if !p.beginSync(public, trustedPeerName(peers, public)) {
		http.Error(w, "another Sync peer is already active", http.StatusConflict)
		return
	}
	query := r.URL.Query()
	size, err := strconv.ParseUint(query.Get("size"), 10, 64)
	if err != nil {
		http.Error(w, "invalid attachment metadata", http.StatusBadRequest)
		return
	}
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            query.Get("file"),
		Name:          query.Get("name"),
		DecryptedSize: size,
		IntegrityHash: query.Get("hash"),
	}
	if err := file.Validate(); err != nil {
		http.Error(w, "invalid attachment metadata", http.StatusBadRequest)
		return
	}
	itemID := query.Get("item")
	switch r.Method {
	case http.MethodHead:
		valid, err := p.store.VerifyAttachment(p.vault, itemID, file)
		if err != nil {
			http.Error(w, "existing attachment does not match metadata", http.StatusConflict)
			return
		}
		if valid {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	case http.MethodGet:
		if err := p.store.WriteAttachment(p.vault, itemID, file, w); err != nil {
			http.Error(w, "could not read attachment", http.StatusInternalServerError)
		}
	case http.MethodPut:
		created, err := p.store.ReceiveAttachment(p.vault, itemID, file, r.Body)
		if err != nil {
			http.Error(w, "could not verify or store attachment", http.StatusConflict)
			return
		}
		if created {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := p.store.CleanupAttachments(p.vault, itemID, []string{file.ID}); err != nil {
			http.Error(w, "could not clean up attachment", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func attachmentURL(address string, file cxf.File, itemID string) string {
	query := url.Values{
		"item": {itemID},
		"file": {file.ID},
		"name": {file.Name},
		"size": {strconv.FormatUint(file.DecryptedSize, 10)},
		"hash": {file.IntegrityHash},
	}
	return syncURL(address, attachmentPath) + "?" + query.Encode()
}

// attachmentTimeout returns a transfer deadline sized for an attachment at the
// slowest supported LAN throughput. It never returns a zero timeout.
func attachmentTimeout(size uint64) time.Duration {
	const maxDuration = time.Duration(math.MaxInt64)
	// Reserve the base timeout so adding it cannot overflow either.
	maxSeconds := uint64((maxDuration - attachmentBaseTimeout) / time.Second)
	// Clamp before converting or multiplying; overflow would make Client.Timeout negative and disable it.
	seconds := min(size/attachmentMinRate, maxSeconds)
	return attachmentBaseTimeout + time.Duration(seconds)*time.Second
}

// attachmentClient returns a client pinned to one peer for an attachment request.
// Attachments can legitimately take longer than the short handshake limit, so the
// timeout grows with the payload size instead of being disabled.
func (c *Client) attachmentClient(public *mldsa.PublicKey, size uint64) (*http.Client, *http.Transport) {
	transport := c.pairingTransport(public)
	client := *c.http
	client.Transport = transport
	client.Timeout = attachmentTimeout(size)
	return &client, transport
}

// transferAttachment streams an attachment between trusted TLS peers without buffering it.
func (p *peer) transferAttachment(
	ctx context.Context,
	file cxf.File,
	itemID string,
	body io.Reader,
) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPut, attachmentURL(p.address, file, itemID), body)
	if err != nil {
		return false, err
	}
	client, transport := p.client.attachmentClient(p.public, file.DecryptedSize)
	defer transport.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return false, fmt.Errorf("transfer attachment: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusCreated {
		return true, nil
	}
	if response.StatusCode == http.StatusNoContent {
		return false, nil
	}
	return false, fmt.Errorf("attachment transfer returned HTTP %d", response.StatusCode)
}

// removeRemoteAttachment removes a newly uploaded blob only if it is still unreferenced.
func (p *peer) removeRemoteAttachment(
	ctx context.Context,
	file cxf.File,
	itemID string,
) error {
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodDelete,
		attachmentURL(p.address, file, itemID),
		nil,
	)
	if err != nil {
		return err
	}
	client, transport := p.client.attachmentClient(p.public, 0)
	defer transport.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("clean up remote attachment: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("remote attachment cleanup returned HTTP %d", response.StatusCode)
	}
	return nil
}

// verifyRemoteAttachment checks whether a matching blob already exists on the peer.
func (p *peer) verifyRemoteAttachment(ctx context.Context, file cxf.File, itemID string) (bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, attachmentURL(p.address, file, itemID), nil)
	if err != nil {
		return false, err
	}
	client, transport := p.client.attachmentClient(p.public, 0)
	defer transport.CloseIdleConnections()
	response, err := client.Do(request)
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusNoContent:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("attachment verification returned HTTP %d", response.StatusCode)
	}
}

// readAttachment returns a verified TLS response body for one attachment.
func (p *peer) readAttachment(ctx context.Context, file cxf.File, itemID string) (io.ReadCloser, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, attachmentURL(p.address, file, itemID), nil)
	if err != nil {
		return nil, err
	}
	client, transport := p.client.attachmentClient(p.public, file.DecryptedSize)
	response, err := client.Do(request)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		transport.CloseIdleConnections()
		return nil, fmt.Errorf("attachment read returned HTTP %d", response.StatusCode)
	}
	return &attachmentResponseBody{ReadCloser: response.Body, transport: transport}, nil
}

type attachmentResponseBody struct {
	io.ReadCloser
	transport *http.Transport
}

func (body *attachmentResponseBody) Close() error {
	err := body.ReadCloser.Close()
	body.transport.CloseIdleConnections()
	return err
}

// serveItem handles trusted item reads and mutations for one unlocked vault.
func (p *pairingServer) serveItem(w http.ResponseWriter, r *http.Request) {
	if p.vault.Key() == nil {
		http.Error(w, "vault is locked", http.StatusLocked)
		return
	}
	remote, err := AuthenticatedPeerKey(r)
	if err != nil {
		http.Error(w, "peer authentication failed", http.StatusUnauthorized)
		return
	}
	peers, err := p.store.ListSyncPeers(p.vault)
	if err != nil {
		http.Error(w, "could not read trusted devices", http.StatusInternalServerError)
		return
	}
	if !isTrustedPeer(peers, remote) {
		http.Error(w, "sync peer is not paired", http.StatusForbidden)
		return
	}
	if !p.beginSync(remote, trustedPeerName(peers, remote)) {
		http.Error(w, "another Sync peer is already active", http.StatusConflict)
		return
	}
	var request itemRequest
	r.Body = http.MaxBytesReader(w, r.Body, maxSyncItemBytes)
	if err := json.UnmarshalRead(r.Body, &request); err != nil {
		http.Error(w, "invalid Sync item request", http.StatusBadRequest)
		return
	}
	if request.ID == "" {
		http.Error(w, "item ID is required", http.StatusBadRequest)
		return
	}
	item, err := checkedItem(r.Context(), p.store, p.vault, request.ID, request.Expected)
	if err != nil {
		http.Error(w, "item changed; rebuild the Sync plan", http.StatusConflict)
		return
	}
	result := itemResponse{}
	switch r.Method {
	case http.MethodPost:
		result.Item = item
	case http.MethodPut:
		if err := validateTransferItem(request.Item, request.ID, request.Source); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := r.Context().Err(); err != nil {
			return
		}
		files, err := storage.AttachmentReferences(request.Item)
		if err != nil {
			http.Error(w, "invalid attachment references", http.StatusBadRequest)
			return
		}
		for _, file := range files {
			valid, err := p.store.VerifyAttachment(p.vault, request.ID, file)
			if err != nil || !valid {
				http.Error(w, "item attachments are missing or invalid", http.StatusConflict)
				return
			}
		}
		if _, err := checkedItem(r.Context(), p.store, p.vault, request.ID, request.Expected); err != nil {
			http.Error(w, "item changed; rebuild the Sync plan", http.StatusConflict)
			return
		}
		progress := SyncProgressEvent{
			DeviceName: trustedPeerName(peers, remote),
			ItemID:     request.ID,
			Title:      request.Item.Title,
			Type:       request.Item.Metadata.Type,
			Started:    true,
		}
		p.reportSyncProgress(progress)
		if err := p.store.StoreItem(p.vault, request.Item); err != nil {
			progress.Started = false
			progress.Failed = true
			p.reportSyncProgress(progress)
			http.Error(w, "could not save Sync item; rebuild the plan", http.StatusInternalServerError)
			return
		}
		progress.Started = false
		progress.Completed = true
		p.reportSyncProgress(progress)
		result.Committed = true
	case http.MethodDelete:
		if item == nil {
			http.Error(w, "item is absent; rebuild the Sync plan", http.StatusConflict)
			return
		}
		if err := r.Context().Err(); err != nil {
			return
		}
		progress := SyncProgressEvent{
			DeviceName: trustedPeerName(peers, remote),
			ItemID:     request.ID,
			Title:      item.Title,
			Type:       item.Metadata.Type,
			Started:    true,
		}
		p.reportSyncProgress(progress)
		if err := p.store.DeleteItem(p.vault, request.ID); err != nil {
			progress.Started = false
			progress.Failed = true
			p.reportSyncProgress(progress)
			http.Error(w, "could not delete Sync item; rebuild the plan", http.StatusInternalServerError)
			return
		}
		progress.Started = false
		progress.Completed = true
		p.reportSyncProgress(progress)
		result.Committed = true
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	data, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "could not encode Sync item", http.StatusInternalServerError)
		return
	}
	if len(data) > maxSyncItemBytes {
		http.Error(w, "Sync item is too large", http.StatusRequestEntityTooLarge)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// transferItem sends one authenticated item request and bounds its response.
func (p *peer) transferItem(ctx context.Context, method string, value itemRequest) (*itemResponse, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(data) > maxSyncItemBytes {
		return nil, fmt.Errorf("sync item is too large")
	}
	request, err := http.NewRequestWithContext(ctx, method, syncURL(p.address, itemPath), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	transport := p.client.pairingTransport(p.public)
	defer transport.CloseIdleConnections()
	client := *p.client.http
	client.Transport = transport
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("sync request interrupted; rebuild the plan: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync item request returned HTTP %d; rebuild the plan", response.StatusCode)
	}
	data, err = io.ReadAll(io.LimitReader(response.Body, maxSyncItemBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxSyncItemBytes {
		return nil, fmt.Errorf("sync item response is too large")
	}
	var result itemResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("invalid Sync item response: %w", err)
	}
	return &result, nil
}

// checkedItem loads the expected logical state. An empty hash requires absence.
func checkedItem(ctx context.Context, store manifestItemStore, vault *model.Vault, id, expected string) (*model.Item, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("unlock the vault before executing Sync")
	}
	if len(vault.Accounts) != 1 {
		return nil, fmt.Errorf("vault account data is invalid")
	}
	exists := false
	for _, summary := range vault.Accounts[0].Items {
		if summary.ID == id {
			exists = true
			break
		}
	}
	if !exists {
		if expected != "" {
			return nil, fmt.Errorf("item changed; rebuild the Sync plan")
		}
		return nil, nil
	}
	if expected == "" {
		return nil, fmt.Errorf("item changed; rebuild the Sync plan")
	}
	item, err := store.LoadItem(vault, id)
	if err != nil {
		return nil, err
	}
	actual, err := item.StateHash()
	if err != nil {
		return nil, err
	}
	if actual != expected {
		return nil, fmt.Errorf("item changed; rebuild the Sync plan")
	}
	return item, nil
}

// validateTransferItem rejects references that this phase cannot transfer.
func validateTransferItem(item *model.Item, id, expected string) error {
	if item == nil {
		return fmt.Errorf("sync item is missing")
	}
	if err := item.Validate(); err != nil {
		return fmt.Errorf("invalid Sync item: %w", err)
	}
	if item.ID != id {
		return fmt.Errorf("sync item ID does not match the plan")
	}
	actual, err := item.StateHash()
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("item changed; rebuild the Sync plan")
	}
	return nil
}
