// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	json "encoding/json/v2"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestBuildDiffClassifiesItemsAndSortsByID(t *testing.T) {
	localModifiedAt := uint64(10)
	remoteModifiedAt := uint64(20)
	manifest := func(id, contentHash, stateHash string) ManifestItem {
		return ManifestItem{
			ID:          id,
			ContentHash: contentHash,
			StateHash:   stateHash,
		}
	}
	local := &Manifest{Items: []ManifestItem{
		{
			ID:          "same",
			Type:        model.LoginItemType,
			Title:       "Local title",
			ModifiedAt:  &localModifiedAt,
			ContentHash: hashByte(1),
			StateHash:   hashByte(1),
		},
		manifest("content", hashByte(2), hashByte(2)),
		manifest("state", hashByte(3), hashByte(3)),
		manifest("local", hashByte(4), hashByte(4)),
	}}
	remote := &Manifest{Items: []ManifestItem{
		manifest("state", hashByte(3), hashByte(5)),
		manifest("content", hashByte(6), hashByte(6)),
		manifest("remote", hashByte(7), hashByte(7)),
		{
			ID:          "same",
			Type:        model.NoteItemType,
			Title:       "Remote title",
			ModifiedAt:  &remoteModifiedAt,
			ContentHash: hashByte(1),
			StateHash:   hashByte(1),
		},
	}}
	diff, err := BuildDiff(local, remote)
	require.NoError(t, err)
	require.Equal(t, []Difference{
		{ItemID: "content", Kind: ContentDifferent, Action: RequiresUserChoice},
		{ItemID: "local", Kind: LocalOnly, Action: CopyToResponder},
		{ItemID: "remote", Kind: RemoteOnly, Action: CopyToPlanner},
		{ItemID: "same", Kind: Identical, Action: NoAction},
		{ItemID: "state", Kind: StateDifferent, Action: RequiresUserChoice},
	}, diff)
	empty, err := BuildDiff(&Manifest{}, &Manifest{})
	require.NoError(t, err)
	require.Empty(t, empty)
}

func TestRemoteManifestUsesTrustedRealVaultsAndDoesNotWrite(t *testing.T) {
	ctx := context.Background()
	plannerStore, plannerVault := pairingVault(t)
	defer plannerStore.Close()
	item, err := model.NewNote("Shared", "same logical content")
	require.NoError(t, err)
	require.NoError(t, plannerStore.StoreItem(plannerVault, item))
	var export bytes.Buffer
	require.NoError(t, storage.WriteCXF(ctx, &export, plannerStore, plannerVault))
	backup, err := storage.ReadPawBackup(ctx, bytes.NewReader(export.Bytes()), "")
	require.NoError(t, err)
	defer backup.Close()
	responderStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer responderStore.Close()
	responderVault, err := responderStore.Restore(ctx, backup, "Twin vault", "different-password")
	require.NoError(t, err)
	require.Equal(t, plannerVault.Accounts[0].ID, responderVault.Accounts[0].ID)
	require.NotEqual(t, plannerVault.Key().String(), responderVault.Key().String())
	localOnly, err := model.NewNote("Planner only", "local")
	require.NoError(t, err)
	require.NoError(t, plannerStore.StoreItem(plannerVault, localOnly))
	remoteOnly, err := model.NewNote("Responder only", "remote")
	require.NoError(t, err)
	require.NoError(t, responderStore.StoreItem(responderVault, remoteOnly))
	plannerPublic, plannerIdentity, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
	require.NoError(t, err)
	responderPublic, responderIdentity, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
	require.NoError(t, err)
	require.NoError(t, plannerStore.AddSyncPeer(plannerVault, storage.SyncPeer{
		PublicKey: responderPublic.Bytes(),
		Name:      "Responder",
	}))
	require.NoError(t, responderStore.AddSyncPeer(responderVault, storage.SyncPeer{
		PublicKey: plannerPublic.Bytes(),
		Name:      "Planner",
	}))
	address, _ := startPairingEndpoint(
		t,
		responderStore,
		responderVault,
		responderPublic,
		responderIdentity,
		"Responder",
	)
	client, err := NewClient(plannerIdentity)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	session := &Session{
		ctx:    sessionCtx,
		client: client,
		store:  plannerStore,
		vault:  plannerVault,
		public: plannerPublic,
	}
	peer := Peer{
		IP:          net.ParseIP("127.0.0.1"),
		Port:        address.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(responderPublic),
	}
	plannerBefore := snapshotStorage(t, plannerStore.Root())
	responderBefore := snapshotStorage(t, responderStore.Root())
	remoteManifest, err := session.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	remoteOnlyManifestItem := manifestItemByID(t, remoteManifest, remoteOnly.ID)
	remoteOnlyItem, err := responderStore.LoadItem(responderVault, remoteOnly.ID)
	require.NoError(t, err)
	require.Equal(t, remoteOnlyItem.Metadata.Type, remoteOnlyManifestItem.Type)
	require.Equal(t, remoteOnlyItem.Title, remoteOnlyManifestItem.Title)
	require.Equal(t, remoteOnlyItem.ModifiedAt, remoteOnlyManifestItem.ModifiedAt)
	localManifest, err := BuildManifest(ctx, plannerStore, plannerVault)
	require.NoError(t, err)
	localManifestItem := manifestItemByID(t, localManifest, localOnly.ID)
	localOnlyItem, err := plannerStore.LoadItem(plannerVault, localOnly.ID)
	require.NoError(t, err)
	require.Equal(t, localOnlyItem.Metadata.Type, localManifestItem.Type)
	require.Equal(t, localOnlyItem.Title, localManifestItem.Title)
	require.Equal(t, localOnlyItem.ModifiedAt, localManifestItem.ModifiedAt)
	require.Len(t, remoteManifest.Items, 2)
	diff, err := BuildDiff(localManifest, remoteManifest)
	require.NoError(t, err)
	require.Len(t, diff, 3)
	byID := make(map[string]DifferenceKind, len(diff))
	for _, difference := range diff {
		byID[difference.ItemID] = difference.Kind
	}
	require.Equal(t, Identical, byID[item.ID])
	require.Equal(t, LocalOnly, byID[localOnly.ID])
	require.Equal(t, RemoteOnly, byID[remoteOnly.ID])
	require.Equal(t, plannerBefore, snapshotStorage(t, plannerStore.Root()))
	require.Equal(t, responderBefore, snapshotStorage(t, responderStore.Root()))
	_, err = responderStore.LoadItem(responderVault, item.ID)
	require.NoError(t, err)

	// The HTTP response is tied to the TLS peer key, and the server checks local trust.
	require.NoError(t, responderStore.RemoveSyncPeer(responderVault, plannerPublic.Bytes()))
	addressString := net.JoinHostPort(address.IP.String(), strconv.Itoa(address.Port))
	_, err = newAuthenticatedPeer(client, addressString, responderPublic).Manifest(ctx)
	require.ErrorContains(t, err, "HTTP 403")
}

func TestManifestJSONRoundTripsPresentationMetadata(t *testing.T) {
	modifiedAt := uint64(1794357000)
	want := Manifest{Items: []ManifestItem{{
		ID:          "metadata-item",
		Type:        model.LoginItemType,
		Title:       "GitHub",
		ModifiedAt:  &modifiedAt,
		ContentHash: hashByte(1),
		StateHash:   hashByte(2),
	}}}
	data, err := json.Marshal(want)
	require.NoError(t, err)
	var got Manifest
	require.NoError(t, json.Unmarshal(data, &got))
	require.Equal(t, want.Items, got.Items)
	require.Contains(t, string(data), `"type":`)
	require.Contains(t, string(data), `"title":"GitHub"`)
	require.Contains(t, string(data), `"modified_at":1794357000`)
}

func manifestItemByID(t *testing.T, manifest *Manifest, id string) ManifestItem {
	t.Helper()
	for _, item := range manifest.Items {
		if item.ID == id {
			return item
		}
	}
	t.Fatalf("manifest item %q not found", id)
	return ManifestItem{}
}

func TestManifestRejectsInvalidInputAndCancellation(t *testing.T) {
	store, vault := pairingVault(t)
	defer store.Close()
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	manifest, err := BuildManifest(canceled, store, vault)
	require.Nil(t, manifest)
	require.ErrorIs(t, err, context.Canceled)
	_, err = BuildDiff(&Manifest{}, &Manifest{Items: []ManifestItem{{
		ID:          "duplicate-or-malformed",
		ContentHash: "invalid",
		StateHash:   "invalid",
	}}})
	require.ErrorContains(t, err, "invalid item hash")
	_, err = BuildDiff(&Manifest{Items: []ManifestItem{
		{ID: "duplicate", ContentHash: hashByte(1), StateHash: hashByte(1)},
		{ID: "duplicate", ContentHash: hashByte(1), StateHash: hashByte(1)},
	}}, &Manifest{})
	require.ErrorContains(t, err, "duplicate item ID")
}

func TestManifestJSONRequiresItemsAndCanonicalHashes(t *testing.T) {
	var manifest Manifest
	err := json.Unmarshal([]byte(`{}`), &manifest)
	require.ErrorContains(t, err, "manifest items are missing")
	err = json.Unmarshal([]byte(`{"items":null}`), &manifest)
	require.ErrorContains(t, err, "manifest items are missing")
	err = json.Unmarshal([]byte(`{"items":[]}`), &manifest)
	require.NoError(t, err)
	require.Empty(t, manifest.Items)
	require.False(t, validManifestHash(strings.ToUpper(hashByte(10))))
	require.True(t, validManifestHash(hashByte(10)))
}

func hashByte(value byte) string {
	data := bytes.Repeat([]byte{value}, sha256.Size)
	return hex.EncodeToString(data)
}

func snapshotStorage(t *testing.T, root string) map[string][sha256.Size]byte {
	t.Helper()
	files := make(map[string][sha256.Size]byte)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = sha256.Sum256(data)
		return nil
	})
	require.NoError(t, err)
	return files
}
