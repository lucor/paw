// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/mldsa"
	"encoding/hex"
	json "encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

const (
	maxManifestBytes = 4 << 20
	maxManifestItems = 10000
)

// Manifest contains item comparison hashes and safe presentation metadata.
type Manifest struct {
	Items []ManifestItem `json:"items"`
}

func (manifest *Manifest) UnmarshalJSON(data []byte) error {
	type manifestJSON struct {
		Items *[]ManifestItem `json:"items"`
	}
	var decoded manifestJSON
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	if decoded.Items == nil {
		return fmt.Errorf("manifest items are missing")
	}
	manifest.Items = *decoded.Items
	return nil
}

// ManifestItem carries comparison hashes and presentation-only item metadata.
type ManifestItem struct {
	ID string `json:"id"`
	// Type and Title are presentation metadata and do not affect comparison.
	Type       model.ItemType `json:"type"`
	Title      string         `json:"title"`
	ModifiedAt *uint64        `json:"modified_at,omitempty"`
	// ContentHash and StateHash remain authoritative for comparison and plan validation.
	ContentHash string `json:"content_hash"`
	StateHash   string `json:"state_hash"`
}

// DifferenceKind describes how an item differs between two manifests.
type DifferenceKind string

const (
	Identical        DifferenceKind = "identical"
	LocalOnly        DifferenceKind = "local_only"
	RemoteOnly       DifferenceKind = "remote_only"
	ContentDifferent DifferenceKind = "content_different"
	StateDifferent   DifferenceKind = "state_different"
)

// SuggestedAction describes a non-binding planner suggestion.
type SuggestedAction string

const (
	NoAction           SuggestedAction = "none"
	CopyToResponder    SuggestedAction = "copy_to_responder"
	CopyToPlanner      SuggestedAction = "copy_to_planner"
	RequiresUserChoice SuggestedAction = "requires_user_choice"
)

// Difference records one deterministic comparison result.
type Difference struct {
	ItemID string          `json:"item_id"`
	Kind   DifferenceKind  `json:"kind"`
	Action SuggestedAction `json:"action"`
}

// BuildManifest reads items through Paw storage and returns their hashes and presentation metadata.
func BuildManifest(
	ctx context.Context,
	store manifestItemStore,
	vault *model.Vault,
) (*Manifest, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil || vault == nil {
		return nil, fmt.Errorf("an unlocked vault is required to build a Sync manifest")
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("an unlocked vault is required to build a Sync manifest")
	}
	if len(vault.Accounts) != 1 {
		return nil, fmt.Errorf("vault account data is invalid")
	}
	if len(vault.Accounts[0].Items) > maxManifestItems {
		return nil, fmt.Errorf("vault has too many items for a Sync manifest")
	}
	manifest := &Manifest{Items: make([]ManifestItem, 0, len(vault.Accounts[0].Items))}
	for _, summary := range vault.Accounts[0].Items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		item, err := store.LoadItem(vault, summary.ID)
		if err != nil {
			return nil, fmt.Errorf("load item for Sync manifest: %w", err)
		}
		contentHash, err := item.ContentHash()
		if err != nil {
			return nil, fmt.Errorf("hash item content for Sync manifest: %w", err)
		}
		stateHash, err := item.StateHash()
		if err != nil {
			return nil, fmt.Errorf("hash item state for Sync manifest: %w", err)
		}
		manifest.Items = append(manifest.Items, ManifestItem{
			ID:          item.ID,
			Type:        item.Metadata.Type,
			Title:       item.Title,
			ModifiedAt:  cloneTimestamp(item.ModifiedAt),
			ContentHash: contentHash,
			StateHash:   stateHash,
		})
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	sortManifest(manifest)
	return manifest, nil
}

func cloneTimestamp(timestamp *uint64) *uint64 {
	if timestamp == nil {
		return nil
	}
	value := *timestamp
	return &value
}

type manifestItemStore interface {
	LoadItem(*model.Vault, string) (*model.Item, error)
}

// BuildDiff classifies items by ID and returns results sorted by ID.
func BuildDiff(local, remote *Manifest) ([]Difference, error) {
	if err := validateManifest(local); err != nil {
		return nil, fmt.Errorf("invalid local manifest: %w", err)
	}
	if err := validateManifest(remote); err != nil {
		return nil, fmt.Errorf("invalid remote manifest: %w", err)
	}
	localItems := make(map[string]ManifestItem, len(local.Items))
	remoteItems := make(map[string]ManifestItem, len(remote.Items))
	ids := make(map[string]struct{}, len(local.Items)+len(remote.Items))
	for _, item := range local.Items {
		localItems[item.ID] = item
		ids[item.ID] = struct{}{}
	}
	for _, item := range remote.Items {
		remoteItems[item.ID] = item
		ids[item.ID] = struct{}{}
	}
	result := make([]Difference, 0, len(ids))
	for id := range ids {
		localItem, hasLocal := localItems[id]
		remoteItem, hasRemote := remoteItems[id]
		difference := Difference{ItemID: id}
		switch {
		case !hasRemote:
			difference.Kind = LocalOnly
			difference.Action = CopyToResponder
		case !hasLocal:
			difference.Kind = RemoteOnly
			difference.Action = CopyToPlanner
		case localItem.ContentHash != remoteItem.ContentHash:
			difference.Kind = ContentDifferent
			difference.Action = RequiresUserChoice
		case localItem.StateHash != remoteItem.StateHash:
			difference.Kind = StateDifferent
			difference.Action = RequiresUserChoice
		default:
			difference.Kind = Identical
			difference.Action = NoAction
		}
		result = append(result, difference)
	}
	slices.SortFunc(result, func(a, b Difference) int {
		return strings.Compare(a.ItemID, b.ItemID)
	})
	return result, nil
}

func (p *pairingServer) serveManifest(w http.ResponseWriter, r *http.Request) {
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
	if !p.beginSync(remote, trustedPeerName(trusted, remote)) {
		http.Error(w, "another Sync peer is already active", http.StatusConflict)
		return
	}
	manifest, err := BuildManifest(r.Context(), p.store, p.vault)
	if err != nil {
		http.Error(w, "could not build vault manifest", http.StatusInternalServerError)
		return
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		http.Error(w, "could not encode vault manifest", http.StatusInternalServerError)
		return
	}
	if len(data) > maxManifestBytes {
		http.Error(w, "vault manifest is too large", http.StatusRequestEntityTooLarge)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// Manifest requests the trusted peer's read-only item manifest.
func (p *peer) Manifest(ctx context.Context) (*Manifest, error) {
	if p == nil || p.client == nil || p.client.http == nil || p.public == nil {
		return nil, fmt.Errorf("sync client and peer identity are required")
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		syncURL(p.address, manifestPath),
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create Sync manifest request: %w", err)
	}
	transport := p.client.pairingTransport(p.public)
	transport.ResponseHeaderTimeout = handshakeLimit
	defer transport.CloseIdleConnections()
	client := *p.client.http
	client.Transport = transport
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("request Sync manifest: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sync manifest request returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxManifestBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read Sync manifest: %w", err)
	}
	if len(data) > maxManifestBytes {
		return nil, fmt.Errorf("sync manifest response is too large")
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode Sync manifest: %w", err)
	}
	if err := validateManifest(&manifest); err != nil {
		return nil, fmt.Errorf("invalid Sync manifest: %w", err)
	}
	sortManifest(&manifest)
	return &manifest, nil
}

func validateManifest(manifest *Manifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is required")
	}
	if len(manifest.Items) > maxManifestItems {
		return fmt.Errorf("manifest contains too many items")
	}
	seen := make(map[string]struct{}, len(manifest.Items))
	for _, item := range manifest.Items {
		if item.ID == "" || len(item.ID) > 128 {
			return fmt.Errorf("manifest contains an invalid item ID")
		}
		if _, exists := seen[item.ID]; exists {
			return fmt.Errorf("manifest contains a duplicate item ID")
		}
		seen[item.ID] = struct{}{}
		if !validManifestHash(item.ContentHash) || !validManifestHash(item.StateHash) {
			return fmt.Errorf("manifest contains an invalid item hash")
		}
	}
	return nil
}

func validManifestHash(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && hex.EncodeToString(decoded) == value
}

func sortManifest(manifest *Manifest) {
	slices.SortFunc(manifest.Items, func(a, b ManifestItem) int {
		return strings.Compare(a.ID, b.ID)
	})
}

func isTrustedPeer(peers []storage.SyncPeer, public *mldsa.PublicKey) bool {
	for _, peer := range peers {
		if bytes.Equal(peer.PublicKey, public.Bytes()) {
			return true
		}
	}
	return false
}

// RemoteManifest retrieves the selected trusted peer's manifest.
func (s *Session) RemoteManifest(ctx context.Context, peer Peer) (*Manifest, error) {
	_, err := s.ConnectTrusted(ctx, peer)
	if err != nil {
		return nil, err
	}
	if s.peer == nil || s.peer.address == "" {
		return nil, fmt.Errorf("sync peer endpoint is unavailable")
	}
	return s.peer.Manifest(ctx)
}
