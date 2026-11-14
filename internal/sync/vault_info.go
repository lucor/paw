// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

const vaultInfoPath = "/sync/vault"

func (p *pairingServer) serveVaultInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if p.vault == nil || p.vault.Key() == nil {
		http.Error(w, "vault is locked", http.StatusLocked)
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
	info, err := vaultReference(p.store, p.vault, p.vaultRef)
	if err != nil {
		http.Error(w, "could not summarize vault", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.MarshalWrite(w, info)
}

func vaultReference(store pairingTrustStore, vault *model.Vault, reference VaultReference) (VaultReference, error) {
	reference.Items = len(vault.Accounts[0].Items)
	reference.Collections = countCollections(vault.Accounts[0].Collections)
	attachments := make(map[string]bool)
	for _, summary := range vault.Accounts[0].Items {
		item, err := store.LoadItem(vault, summary.ID)
		if err != nil {
			return VaultReference{}, fmt.Errorf("load item for Clone summary: %w", err)
		}
		files, err := storage.AttachmentReferences(item)
		if err != nil {
			return VaultReference{}, fmt.Errorf("read attachments for Clone summary: %w", err)
		}
		for _, file := range files {
			attachments[file.ID] = true
		}
	}
	reference.Attachments = len(attachments)
	return reference, nil
}

func countCollections(collections []cxf.Collection) int {
	count := len(collections)
	for _, collection := range collections {
		count += countCollections(collection.SubCollections)
	}
	return count
}

// VaultInfo retrieves the paired peer's vault name and ID.
func (p *peer) VaultInfo(ctx context.Context) (VaultReference, error) {
	if p == nil || p.client == nil || p.client.http == nil || p.public == nil {
		return VaultReference{}, fmt.Errorf("sync client and peer identity are required")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, syncURL(p.address, vaultInfoPath), nil)
	if err != nil {
		return VaultReference{}, fmt.Errorf("create vault info request: %w", err)
	}
	transport := p.client.pairingTransport(p.public)
	defer transport.CloseIdleConnections()
	client := *p.client.http
	client.Transport = transport
	response, err := client.Do(request)
	if err != nil {
		return VaultReference{}, fmt.Errorf("request vault info: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return VaultReference{}, fmt.Errorf("vault info request returned HTTP %d", response.StatusCode)
	}
	var info VaultReference
	if err := json.UnmarshalRead(io.LimitReader(response.Body, 4096), &info); err != nil {
		return VaultReference{}, fmt.Errorf("decode vault info: %w", err)
	}
	return info, nil
}
