// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestStoreCXFItem checks creation and editing preserve full items and derived summaries.
func TestStoreCXFItem(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	key := vault.Key()
	vaultExtension := jsontext.Value(`{"name":"example.com/vault","value":"Preserved vault data"}`)
	vault.Accounts[0].Extensions = append(vault.Accounts[0].Extensions, vaultExtension)
	vault.Timestamp = 1
	item, err := model.NewNote("Original title", "Synthetic note secret")
	require.NoError(t, err)
	created := uint64(1700000000)
	item.CreationAt, item.ModifiedAt = &created, &created
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/item","secret":"Private extension value"}`))
	item.Credentials = append(item.Credentials, jsontext.Value(`{"type":"example-credential","value":"Private credential value"}`))
	start := uint64(time.Now().Unix())
	require.NoError(t, storage.StoreItem(vault, item))
	require.GreaterOrEqual(t, vault.Timestamp, start)
	require.LessOrEqual(t, vault.Timestamp, uint64(time.Now().Unix()))
	require.Same(t, key, vault.Key())
	assertStoredCXFItem(t, storage, vault, item)

	other, err := model.NewNote("Other note", "Other content")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, other))
	otherSummary := vault.Accounts[0].Items[1]
	item.Title = "Renamed title"
	modified := created + 60
	item.ModifiedAt = &modified
	replacement, err := model.NewNote("Unused title", "Edited note secret")
	require.NoError(t, err)
	item.Credentials[0] = replacement.Credentials[0]
	require.NoError(t, storage.StoreItem(vault, item))
	require.Len(t, vault.Accounts[0].Items, 2)
	require.Equal(t, otherSummary, vault.Accounts[0].Items[1])
	require.Equal(t, created, *item.CreationAt)
	history, err := item.History()
	require.NoError(t, err)
	require.Empty(t, history.Entries, "StoreItem must not create user-edit history")
	assertStoredCXFItem(t, storage, vault, item)
	decoded := readStoredCXFVault(t, storage, vault)
	require.Contains(t, decoded.Accounts[0].Extensions, vaultExtension)
	require.Equal(t, vault.Header, decoded.Header)
}

func TestAddAttachmentCommitsBlobAndHistory(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("Attachment note", "content")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, item))
	file, err := storage.AddAttachment(vault, item.ID, "same name.bin", bytes.NewReader([]byte{0, 255, 1}))
	require.NoError(t, err)
	loaded, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	files, err := loaded.Files()
	require.NoError(t, err)
	require.Equal(t, []cxf.File{file}, files)
	listed, err := storage.ListAttachments(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, []cxf.File{file}, listed)
	directory, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(directory, "attachment.bin")
	beforeHash, err := loaded.StateHash()
	require.NoError(t, err)
	err = storage.SaveAttachment(vault, item.ID, file.ID, destination)
	require.NoError(t, err)
	content, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, []byte{0, 255, 1}, content)
	after, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	afterHash, err := after.StateHash()
	require.NoError(t, err)
	require.Equal(t, beforeHash, afterHash)
	history, err := loaded.History()
	require.NoError(t, err)
	require.Len(t, history.Entries, 1)
	require.Equal(t, item.ID, history.Entries[0].Item.ID)
	require.NoError(t, storage.RemoveAttachment(vault, item.ID, file.ID))
	removed, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	files, err = removed.Files()
	require.NoError(t, err)
	require.Empty(t, files)
	history, err = removed.History()
	require.NoError(t, err)
	require.Len(t, history.Entries, 2)
	root, err := storage.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	attachmentPath := filepath.Join(itemsDirectoryName, filesystemID(item.ID), filesDirectoryName, attachmentFilename(file.ID))
	_, err = root.Stat(attachmentPath)
	require.NoError(t, err, "history still references the encrypted blob")
	require.NoError(t, root.Close())
	for i := range 10 {
		previous, err := storage.LoadItem(vault, item.ID)
		require.NoError(t, err)
		proposed := *previous
		proposed.Title = fmt.Sprintf("Attachment note %d", i)
		now := uint64(time.Now().Unix())
		updated, changed, err := model.PrepareItemEdit(previous, &proposed, now, now)
		require.NoError(t, err)
		require.True(t, changed)
		require.NoError(t, storage.StoreItem(vault, updated))
	}
	root, err = storage.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	_, err = root.Stat(attachmentPath)
	require.ErrorIs(t, err, os.ErrNotExist, "blob is removed after its last history reference expires")
	require.NoError(t, root.Close())
}

func TestAddAttachmentCommitFailureKeepsPreviousItemUsable(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("commit failure", "old content")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, item))
	before, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	beforeHash, err := before.StateHash()
	require.NoError(t, err)
	vaultFile := vaultPath(storage, vault.Accounts[0].ID)
	blocked := vaultFile + ".blocked"
	require.NoError(t, os.Rename(vaultFile, blocked))
	require.NoError(t, os.Mkdir(vaultFile, 0700))
	_, addErr := storage.AddAttachment(vault, item.ID, "orphan.bin", bytes.NewReader([]byte("payload")))
	require.Error(t, addErr)
	require.NoError(t, os.Remove(vaultFile))
	require.NoError(t, os.Rename(blocked, vaultFile))
	after, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	afterHash, err := after.StateHash()
	require.NoError(t, err)
	require.Equal(t, beforeHash, afterHash)
	files, err := after.Files()
	require.NoError(t, err)
	require.Empty(t, files)
	root, err := storage.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	filesDir := filepath.Join(itemsDirectoryName, filesystemID(item.ID), filesDirectoryName)
	entries := readAttachmentEntries(t, root, filesDir)
	require.Len(t, entries, 1, "failed commit may leave one unreferenced encrypted blob")
	require.NoError(t, root.Close())
}

func TestRemoveAttachmentCommitFailureKeepsReferenceAndBlob(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("remove failure", "note")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, item))
	file, err := storage.AddAttachment(vault, item.ID, "file.bin", bytes.NewReader([]byte("payload")))
	require.NoError(t, err)
	vaultFile := vaultPath(storage, vault.Accounts[0].ID)
	blocked := vaultFile + ".blocked"
	require.NoError(t, os.Rename(vaultFile, blocked))
	require.NoError(t, os.Mkdir(vaultFile, 0700))
	require.Error(t, storage.RemoveAttachment(vault, item.ID, file.ID))
	require.NoError(t, os.Remove(vaultFile))
	require.NoError(t, os.Rename(blocked, vaultFile))
	current, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	files, err := current.Files()
	require.NoError(t, err)
	require.Equal(t, []cxf.File{file}, files)
	root, err := storage.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	path := filepath.Join(itemsDirectoryName, filesystemID(item.ID), filesDirectoryName, attachmentFilename(file.ID))
	info, err := root.Stat(path)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular())
	require.NoError(t, root.Close())
}

func TestStoreItemHashNeverEntersCompleteItem(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("Hashed summary", "synthetic content")
	require.NoError(t, err)
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"paw.pm/item-hash","algorithm":"sha256","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	require.NoError(t, storage.StoreItem(vault, item))

	fullItem := readStoredCXFItem(t, storage, vault, item.ID)
	fullItemJSON, err := json.Marshal(fullItem, json.Deterministic(true))
	require.NoError(t, err)
	require.NotContains(t, string(fullItemJSON), "paw.pm/item-hash")
	history, err := fullItem.History()
	require.NoError(t, err)
	require.Empty(t, history.Entries)

	storedVault := readStoredCXFVault(t, storage, vault)
	var hashFound bool
	for _, extension := range storedVault.Accounts[0].Items[0].Extensions {
		var decoded struct {
			Name string `json:"name"`
			Hash string `json:"hash"`
		}
		require.NoError(t, json.Unmarshal(extension, &decoded))
		if decoded.Name == "paw.pm/item-hash" {
			hashFound = true
			want, err := fullItem.StateHash()
			require.NoError(t, err)
			require.Equal(t, want, decoded.Hash)
		}
	}
	require.True(t, hashFound)
}

func TestStoreItemEditsPreserveHistoryOrderAndStateHash(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("Original", "synthetic content")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, item))

	edits := []struct {
		title      string
		capturedAt uint64
	}{
		{
			title:      "First edit",
			capturedAt: 30,
		},
		{
			title:      "Second edit",
			capturedAt: 20,
		},
	}
	for i, edit := range edits {
		previous, err := storage.LoadItem(vault, item.ID)
		require.NoError(t, err)
		proposed := *previous
		proposed.Title = edit.title
		modifiedAt := uint64(40 + i)
		updated, changed, err := model.PrepareItemEdit(previous, &proposed, edit.capturedAt, modifiedAt)
		require.NoError(t, err)
		require.True(t, changed)
		require.NoError(t, storage.StoreItem(vault, updated))
	}

	reopened, err := storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	history, err := reopened.History()
	require.NoError(t, err)
	require.Len(t, history.Entries, 2)
	require.Equal(t, "Original", history.Entries[0].Item.Title)
	require.Equal(t, "First edit", history.Entries[1].Item.Title)
	require.Equal(t, uint64(30), history.Entries[0].CapturedAt)
	require.Equal(t, uint64(20), history.Entries[1].CapturedAt)

	stateHash, err := reopened.StateHash()
	require.NoError(t, err)
	unchangedProposal := *reopened
	_, changed, err := model.PrepareItemEdit(reopened, &unchangedProposal, 99, 99)
	require.NoError(t, err)
	require.False(t, changed)
	require.NoError(t, storage.StoreItem(vault, reopened))
	reopened, err = storage.LoadItem(vault, item.ID)
	require.NoError(t, err)
	reopenedHash, err := reopened.StateHash()
	require.NoError(t, err)
	require.Equal(t, stateHash, reopenedHash)
	reopenedHistory, err := reopened.History()
	require.NoError(t, err)
	require.Equal(t, history, reopenedHistory)
	assertStoredCXFItem(t, storage, vault, reopened)
}

// TestStoreCXFItemFailures checks file errors preserve item-first write order and memory.
func TestStoreCXFItemFailures(t *testing.T) {
	for _, phase := range []string{"item", "vault"} {
		t.Run(phase, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			item, err := model.NewNote("Original", "Original content")
			require.NoError(t, err)
			require.NoError(t, storage.StoreItem(vault, item))
			vault.Metadata.VaultName = "Unsaved name"
			before, err := json.Marshal(&vault.Header, json.Deterministic(true))
			require.NoError(t, err)
			vaultFile := vaultPath(storage, vault.Accounts[0].ID)
			beforeFile, err := os.ReadFile(vaultFile)
			require.NoError(t, err)
			item.Title = "Edited title"
			failingPath := itemPath(storage, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
			if phase == "vault" {
				failingPath = vaultFile
			}
			if phase == "vault" {
				require.NoError(t, os.Remove(failingPath))
			}
			require.NoError(t, os.Mkdir(failingPath, 0700))
			err = storage.StoreItem(vault, item)
			require.Error(t, err)
			if phase == "item" {
				require.ErrorContains(t, err, "collision")
				actual, err := os.ReadFile(vaultFile)
				require.NoError(t, err)
				require.Equal(t, beforeFile, actual)
			} else {
				require.ErrorContains(t, err, "after item was saved")
				actual := readStoredCXFItem(t, storage, vault, item.ID)
				require.Equal(t, "Original", actual.Title)
			}
			after, err := json.Marshal(&vault.Header, json.Deterministic(true))
			require.NoError(t, err)
			require.Equal(t, before, after)
			require.Equal(t, "Unsaved name", vault.Metadata.VaultName)
		})
	}
}

// TestStoreCXFItemValidatesBeforeWriting checks invalid inputs never open files.
func TestStoreCXFItemValidatesBeforeWriting(t *testing.T) {
	for _, invalid := range []string{"item", "vault", "key"} {
		t.Run(invalid, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			vaultFile := vaultPath(storage, vault.Accounts[0].ID)
			before, err := os.ReadFile(vaultFile)
			require.NoError(t, err)
			item, err := model.NewNote("Title", "Content")
			require.NoError(t, err)
			switch invalid {
			case "item":
				item.ID = "invalid ID"
			case "vault":
				vault.Accounts = nil
			case "key":
				vault = &model.Vault{Header: vault.Header, Metadata: vault.Metadata}
			}
			err = storage.storeItem(vault, item)
			require.Error(t, err)
			after, err := os.ReadFile(vaultFile)
			require.NoError(t, err)
			require.Equal(t, before, after)
			files, err := os.ReadDir(filepath.Dir(vaultFile))
			require.NoError(t, err)
			require.Len(t, files, 1)
		})
	}
}

// TestDeleteCXFItem checks both backends remove an item and its vault summary.
func TestDeleteCXFItem(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	first, err := model.NewNote("First", "First content")
	require.NoError(t, err)
	second, err := model.NewNote("Second", "Second content")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, first))
	require.NoError(t, storage.StoreItem(vault, second))
	key := vault.Key()
	start := uint64(time.Now().Unix())

	require.NoError(t, storage.DeleteItem(vault, first.ID))
	require.Same(t, key, vault.Key())
	require.GreaterOrEqual(t, vault.Timestamp, start)
	require.Len(t, vault.Accounts[0].Items, 1)
	require.Equal(t, second.ID, vault.Accounts[0].Items[0].ID)
	require.NoFileExists(t, itemPath(storage, vault.Accounts[0].ID, first.ID, testItemStateHash(t, first)))
	require.FileExists(t, itemPath(storage, vault.Accounts[0].ID, second.ID, testItemStateHash(t, second)))
	decoded := readStoredCXFVault(t, storage, vault)
	require.Len(t, decoded.Accounts[0].Items, 1)
	require.Equal(t, second.ID, decoded.Accounts[0].Items[0].ID)
}

// TestDeleteCXFItemFailures checks item-first deletion and memory update behavior.
func TestDeleteCXFItemFailures(t *testing.T) {
	for _, phase := range []string{"item", "vault"} {
		t.Run(phase, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			item, err := model.NewNote("Title", "Content")
			require.NoError(t, err)
			require.NoError(t, storage.StoreItem(vault, item))
			before, err := json.Marshal(&vault.Header, json.Deterministic(true))
			require.NoError(t, err)
			vaultFile := vaultPath(storage, vault.Accounts[0].ID)
			itemFile := itemPath(storage, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
			failingPath := itemFile
			if phase == "vault" {
				failingPath = vaultFile
			}
			require.NoError(t, os.Remove(failingPath))
			require.NoError(t, os.Mkdir(failingPath, 0700))
			if phase == "item" {
				require.NoError(t, os.WriteFile(filepath.Join(failingPath, "blocker"), nil, 0600))
			}

			err = storage.DeleteItem(vault, item.ID)
			if phase == "item" {
				require.NoError(t, err)
				require.Empty(t, vault.Accounts[0].Items)
				require.FileExists(t, filepath.Join(failingPath, "blocker"))
				return
			} else {
				require.Error(t, err)
				require.ErrorContains(t, err, "after item was deleted")
				require.FileExists(t, itemFile)
			}
			after, marshalErr := json.Marshal(&vault.Header, json.Deterministic(true))
			require.NoError(t, marshalErr)
			require.Equal(t, before, after)
		})
	}
}

// TestDeleteCXFItemMissingSummary checks unknown items are rejected before file changes.
func TestDeleteCXFItemMissingSummary(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	err := storage.DeleteItem(vault, "AAAAAAAAAAAAAAAAAAAAAA")
	require.ErrorContains(t, err, "is not in the vault")
	require.Empty(t, vault.Accounts[0].Items)
}

// TestPrepareItemVaultIsolated checks summary updates and serialization cannot mutate the source.
func TestPrepareItemVaultIsolated(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("Original", "Content")
	require.NoError(t, err)
	require.NoError(t, storage.storeItem(vault, item))
	vault.Metadata.VaultName = "Updated name"
	item.Title = "Updated title"
	before, err := json.Marshal(&vault.Header, json.Deterministic(true))
	require.NoError(t, err)
	updated, err := prepareItemVault(vault, item)
	require.NoError(t, err)
	_, err = json.Marshal(updated, json.Deterministic(true))
	require.NoError(t, err)
	after, err := json.Marshal(&vault.Header, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, "Updated title", updated.Accounts[0].Items[0].Title)
	require.Equal(t, "Original", vault.Accounts[0].Items[0].Title)
	require.Same(t, vault.Key(), updated.Key())
}

// newItemStoreVault creates an encrypted vault without password generation overhead.
func newItemStoreVault(t *testing.T) (*FileStorage, *model.Vault) {
	t.Helper()
	storage := newTestFileStorage(t)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(vaultRootPath(storage, vault.Accounts[0].ID), 0700))
	root, err := storage.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	require.NoError(t, storage.storeEncryptedInRoot(root, vaultFileName, key, vault))
	require.NoError(t, root.Close())
	return storage, vault
}

// assertStoredCXFItem checks full credentials stay in the item and out of its summary.
func assertStoredCXFItem(t *testing.T, storage *FileStorage, vault *model.Vault, item *model.Item) {
	t.Helper()
	actual := readStoredCXFItem(t, storage, vault, item.ID)
	require.Equal(t, item.Item, actual.Item)
	require.Equal(t, item.Metadata, actual.Metadata)
	decoded := readStoredCXFVault(t, storage, vault)
	summary, err := item.Summary()
	require.NoError(t, err)
	storedSummary := decoded.Accounts[0].Items[0]
	require.Equal(t, *summary, cxfItemWithoutHash(t, storedSummary))
	require.Empty(t, storedSummary.Credentials)
	var storedHash string
	for _, extension := range storedSummary.Extensions {
		var metadata struct {
			Name      string `json:"name"`
			Algorithm string `json:"algorithm"`
			Hash      string `json:"hash"`
		}
		require.NoError(t, json.Unmarshal(extension, &metadata))
		if metadata.Name == "paw.pm/item-hash" {
			require.Equal(t, "sha256", metadata.Algorithm)
			storedHash = metadata.Hash
		}
	}
	wantHash, err := item.StateHash()
	require.NoError(t, err)
	require.Equal(t, wantHash, storedHash)
	require.Len(t, storedHash, 64)
	persistedItem := readStoredCXFItem(t, storage, vault, item.ID)
	persistedHash, err := persistedItem.StateHash()
	require.NoError(t, err)
	require.Equal(t, storedHash, persistedHash)
	fullItemData, err := json.Marshal(persistedItem, json.Deterministic(true))
	require.NoError(t, err)
	require.NotContains(t, string(fullItemData), "paw.pm/item-hash")
	data, err := json.Marshal(&decoded.Header, json.Deterministic(true))
	require.NoError(t, err)
	require.NotContains(t, string(data), "Synthetic note secret")
	require.NotContains(t, string(data), "Private extension value")
	require.NotContains(t, string(data), "Private credential value")
}

func cxfItemWithoutHash(t *testing.T, item cxf.Item) cxf.Item {
	t.Helper()
	filtered := item
	filtered.Extensions = make([]jsontext.Value, 0, len(item.Extensions))
	for _, extension := range item.Extensions {
		var header struct {
			Name string `json:"name"`
		}
		require.NoError(t, json.Unmarshal(extension, &header))
		if header.Name != "paw.pm/item-hash" {
			filtered.Extensions = append(filtered.Extensions, extension)
		}
	}
	return filtered
}

// readStoredCXFItem decrypts an item file using the same full-stream reader as storage.
func readStoredCXFItem(t *testing.T, storage *FileStorage, vault *model.Vault, id string) *model.Item {
	t.Helper()
	data, err := os.ReadFile(itemPath(storage, vault.Accounts[0].ID, id, testSummaryStateHash(t, vault, id)))
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("age-encryption.org/v1")))
	item, err := decryptItem(vault.Key(), bytes.NewReader(data), id)
	require.NoError(t, err)
	return item
}

// readStoredCXFVault decrypts the saved vault without reading its item files.
func readStoredCXFVault(t *testing.T, storage *FileStorage, vault *model.Vault) *model.Vault {
	t.Helper()
	data, err := os.ReadFile(vaultPath(storage, vault.Accounts[0].ID))
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("age-encryption.org/v1")))
	decoded, err := decryptVault(vault.Key(), bytes.NewReader(data))
	require.NoError(t, err)
	return decoded
}
