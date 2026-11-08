// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
)

func TestRestoreItemPersistsSelectedVersionAndCanBeReversed(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("A", "synthetic A")
	require.NoError(t, err)
	item.Metadata.Favicon = &model.Favicon{
		Host:   "a.example",
		Data:   []byte("icon A"),
		Format: "png",
	}
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/future","value":"preserved"}`))
	require.NoError(t, store.StoreItem(vault, item))
	creationAt := *item.CreationAt

	for i, edit := range []struct {
		title   string
		content string
	}{
		{
			title:   "B",
			content: "synthetic B",
		},
		{
			title:   "C",
			content: "synthetic C",
		},
		{
			title:   "D",
			content: "synthetic D",
		},
	} {
		previous, err := store.LoadItem(vault, item.ID)
		require.NoError(t, err)
		proposedData, err := json.Marshal(previous, json.Deterministic(true))
		require.NoError(t, err)
		var proposed model.Item
		require.NoError(t, json.Unmarshal(proposedData, &proposed))
		setSyntheticNote(t, &proposed, edit.title, edit.content)
		modifiedAt := uint64(100 + i)
		updated, changed, err := model.PrepareItemEdit(previous, &proposed, modifiedAt, modifiedAt)
		require.NoError(t, err)
		require.True(t, changed)
		require.NoError(t, store.StoreItem(vault, updated))
	}

	current, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	beforeHistory, err := current.History()
	require.NoError(t, err)
	require.Len(t, beforeHistory.Entries, 3)
	selected := beforeHistory.Entries[1]

	restored, changed, err := store.RestoreItem(vault, item.ID, 1, &selected, 200)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "B", restored.Title)
	require.Equal(t, item.ID, restored.ID)
	require.Equal(t, creationAt, *restored.CreationAt)
	require.Equal(t, uint64(200), *restored.ModifiedAt)
	var restoredNote cxf.Note
	require.NoError(t, json.Unmarshal(restored.Credentials[0], &restoredNote))
	require.Equal(t, "synthetic B", restoredNote.Content.Value)
	require.Equal(t, &model.Favicon{
		Host:   "a.example",
		Data:   []byte("icon A"),
		Format: "png",
	}, restored.Metadata.Favicon)
	require.Contains(t, string(restored.Extensions[1]), "example.com/future")
	restoredHistory, err := restored.History()
	require.NoError(t, err)
	require.Len(t, restoredHistory.Entries, 4)
	require.Equal(t, beforeHistory.Entries, restoredHistory.Entries[:3])
	require.Equal(t, selected, restoredHistory.Entries[1])
	require.Equal(t, "D", restoredHistory.Entries[3].Item.Title)
	assertStoredCXFItem(t, store, vault, restored)

	selectedD := restoredHistory.Entries[3]
	reverted, changed, err := store.RestoreItem(vault, item.ID, 3, &selectedD, 201)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "D", reverted.Title)
	revertedHistory, err := reverted.History()
	require.NoError(t, err)
	require.Len(t, revertedHistory.Entries, 5)
	require.Equal(t, "B", revertedHistory.Entries[4].Item.Title)
	require.Equal(t, selectedD, revertedHistory.Entries[3])
	assertStoredCXFItem(t, store, vault, reverted)

	reopenedVault, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	reopenedItem, err := store.LoadItem(reopenedVault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "D", reopenedItem.Title)
	reopenedHistory, err := reopenedItem.History()
	require.NoError(t, err)
	require.Equal(t, revertedHistory, reopenedHistory)
}

func TestRestoreItemNoOpDoesNotWrite(t *testing.T) {
	store, vault, item := newItemWithOneHistoryEntry(t)
	expected, err := item.History()
	require.NoError(t, err)
	entry := expected.Entries[0]
	itemPath := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
	vaultPath := vaultPath(store, vault.Accounts[0].ID)
	itemBytes, err := os.ReadFile(itemPath)
	require.NoError(t, err)
	vaultBytes, err := os.ReadFile(vaultPath)
	require.NoError(t, err)

	restored, changed, err := store.RestoreItem(vault, item.ID, 0, &entry, 50)
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, item.Title, restored.Title)
	afterItemBytes, err := os.ReadFile(itemPath)
	require.NoError(t, err)
	afterVaultBytes, err := os.ReadFile(vaultPath)
	require.NoError(t, err)
	require.True(t, bytes.Equal(itemBytes, afterItemBytes))
	require.True(t, bytes.Equal(vaultBytes, afterVaultBytes))
	history, err := restored.History()
	require.NoError(t, err)
	require.Len(t, history.Entries, 1)
}

func TestRestoreItemRejectsStaleSelection(t *testing.T) {
	store, vault, item := newItemWithOneHistoryEntry(t)
	history, err := item.History()
	require.NoError(t, err)
	staleSelection := history.Entries[0]
	history.Entries[0].Item.Title = "Changed historical snapshot"
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))

	_, changed, err := store.RestoreItem(vault, item.ID, 0, &staleSelection, 60)
	require.ErrorContains(t, err, "stale")
	require.False(t, changed)
}

func TestRestoreItemRejectsMissingVaultItemAndHistory(t *testing.T) {
	store, vault := newItemStoreVault(t)
	_, changed, err := store.RestoreItem(nil, "item", 0, nil, 1)
	require.ErrorContains(t, err, "vault is required")
	require.False(t, changed)
	_, changed, err = store.RestoreItem(vault, "AAAAAAAAAAAAAAAAAAAAAA", 0, nil, 1)
	require.ErrorContains(t, err, "item is not in the vault")
	require.False(t, changed)

	item, err := model.NewNote("No history", "synthetic")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	_, changed, err = store.RestoreItem(vault, item.ID, 0, nil, 1)
	require.ErrorContains(t, err, "no history")
	require.False(t, changed)
}

func TestRestoreItemRetentionAndFailedVaultWrite(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Current", "synthetic current")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		previous, err := model.NewNote("Old", "synthetic old")
		require.NoError(t, err)
		previous.ID = item.ID
		previous.Title = fmt.Sprintf("Version %02d", i+1)
		require.NoError(t, history.AddSnapshot(previous, uint64(i+1)))
	}
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	selected := history.Entries[5]

	vaultFile := vaultPath(store, vault.Accounts[0].ID)
	beforeHeader, err := json.Marshal(vault.Header, json.Deterministic(true))
	require.NoError(t, err)
	require.NoError(t, os.Remove(vaultFile))
	require.NoError(t, os.Mkdir(vaultFile, 0700))

	_, changed, err := store.RestoreItem(vault, item.ID, 5, &selected, 100)
	require.Error(t, err)
	require.False(t, changed)
	afterHeader, marshalErr := json.Marshal(vault.Header, json.Deterministic(true))
	require.NoError(t, marshalErr)
	require.Equal(t, beforeHeader, afterHeader)
	require.Equal(t, selected, history.Entries[5], "caller-owned history stays unchanged on persistence failure")

	persisted, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", persisted.Title)
	persistedHistory, err := persisted.History()
	require.NoError(t, err)
	require.Len(t, persistedHistory.Entries, 10)
	require.Equal(t, history, persistedHistory)
}

func newItemWithOneHistoryEntry(t *testing.T) (*FileStorage, *model.Vault, *model.Item) {
	t.Helper()
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Same", "synthetic same")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 10))
	modifiedAt := uint64(20)
	item.ModifiedAt = &modifiedAt
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	loaded, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	return store, vault, loaded
}

func setSyntheticNote(t *testing.T, item *model.Item, title, content string) {
	t.Helper()
	var note cxf.Note
	require.NoError(t, json.Unmarshal(item.Credentials[0], &note))
	item.Title = title
	note.Content.Value = content
	encoded, err := json.Marshal(note, json.Deterministic(true))
	require.NoError(t, err)
	item.Credentials[0] = encoded
}
