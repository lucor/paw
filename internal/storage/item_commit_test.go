// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json/jsontext"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

func TestItemCommitCrashRecovery(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "before commit", true: "after commit"}[committed], func(t *testing.T) {
			store, vault := newItemStoreVault(t)
			item := newMigrationTestItem(t, store, vault, "Old")
			oldPath := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
			item.Title = "New"
			hash := testItemStateHash(t, item)
			root, err := store.openVaultRoot(vault.Accounts[0].ID)
			require.NoError(t, err)
			// Simulate termination after writing the new version, with no cleanup.
			require.NoError(t, store.storeEncryptedInRoot(root, itemRelativePath(item.ID, hash), vault.Key(), item))
			if committed {
				updated, err := prepareItemVault(vault, item)
				require.NoError(t, err)
				require.NoError(t, store.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated))
			}
			require.NoError(t, root.Close())
			for range 2 {
				loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
				require.NoError(t, err)
				actual, err := store.LoadItem(loaded, item.ID)
				require.NoError(t, err)
				if committed {
					require.Equal(t, "New", actual.Title)
					require.NoFileExists(t, oldPath)
				} else {
					require.Equal(t, "Old", actual.Title)
					require.FileExists(t, oldPath)
					require.NoFileExists(t, itemPath(store, vault.Accounts[0].ID, item.ID, hash))
				}
			}
		})
	}
}

func TestDeleteItemCleansAttachmentsAfterCommitAndPreservesUnknownFiles(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("delete attachment owner", "note")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	file, err := store.AddAttachment(vault, item.ID, "data.bin", bytes.NewReader([]byte("data")))
	require.NoError(t, err)
	root, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	dir := filepath.Join(itemsDirectoryName, filesystemID(item.ID), filesDirectoryName)
	require.NoError(t, root.WriteFile(filepath.Join(dir, "unknown!.age"), []byte("preserve"), 0600))
	staged := filepath.Join(dir, ".paw-attachment-0123456789abcdef01234567.tmp")
	require.NoError(t, root.WriteFile(staged, []byte("encrypted staging"), 0600))
	require.NoError(t, root.Close())
	require.NoError(t, store.DeleteItem(vault, item.ID))
	root, err = store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	_, err = root.Stat(filepath.Join(dir, attachmentFilename(file.ID)))
	require.ErrorIs(t, err, os.ErrNotExist)
	_, err = root.Stat(staged)
	require.ErrorIs(t, err, os.ErrNotExist)
	data, err := root.ReadFile(filepath.Join(dir, "unknown!.age"))
	require.NoError(t, err)
	require.Equal(t, []byte("preserve"), data)
	require.NoError(t, root.Close())
}

func TestInterruptedItemDeletionPreservesUnknownFiles(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item := newMigrationTestItem(t, store, vault, "Deleted")
	path := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
	unknown := filepath.Join(filepath.Dir(path), "unknown.txt")
	require.NoError(t, os.WriteFile(unknown, []byte("preserve"), 0600))
	root, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	updated, err := prepareItemDeleteVault(vault, item.ID)
	require.NoError(t, err)
	require.NoError(t, store.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated))
	require.NoError(t, root.Close())
	for range 2 {
		loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
		require.NoError(t, err)
		require.Empty(t, loaded.Accounts[0].Items)
		require.NoFileExists(t, path)
		require.FileExists(t, unknown)
	}
}

func TestItemVersionsAreImmutableAndPreserveVaultMetadata(t *testing.T) {
	store, vault := newItemStoreVault(t)
	collectionID, err := model.NewNote("Collection identity", "")
	require.NoError(t, err)
	vault.Accounts[0].Collections = []cxf.Collection{{
		ID:    collectionID.ID,
		Title: "Preserved",
		Items: []cxf.LinkedItem{},
	}}
	vault.Accounts[0].Extensions = append(vault.Accounts[0].Extensions, jsontext.Value(`{"name":"example.com/test","value":42}`))
	item := newMigrationTestItem(t, store, vault, "First")
	for _, title := range []string{"Second", "Third"} {
		old := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
		item.Title = title
		require.NoError(t, store.StoreItem(vault, item))
		require.NoFileExists(t, old)
	}
	path := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	require.Equal(t, vault.Accounts[0].Collections, loaded.Accounts[0].Collections)
	require.Contains(t, loaded.Accounts[0].Extensions, jsontext.Value(`{"name":"example.com/test","value":42}`))
}

func TestInvalidSummaryHashRejectsLoadAndMigration(t *testing.T) {
	for _, hash := range []string{"", "../escape", strings.Repeat("A", 64)} {
		store, vault := newItemStoreVault(t)
		item := newMigrationTestItem(t, store, vault, "Test")
		vault.Accounts[0].Items[0].Extensions = []jsontext.Value{jsontext.Value(`{"name":"paw.pm/item-hash","algorithm":"sha256","hash":"` + hash + `"}`)}
		_, err := store.LoadItem(vault, item.ID)
		require.Error(t, err)
		require.Error(t, store.migrateItemLayout(vault))
	}
}

func TestUnsupportedIntermediateLayoutAndImmutableCollision(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item := newMigrationTestItem(t, store, vault, "Original")
	path := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	intermediate := filepath.Join(filepath.Dir(path), "item.age")
	require.NoError(t, os.Rename(path, intermediate))
	_, err = store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.ErrorContains(t, err, "has no committed storage file")
	after, err := os.ReadFile(intermediate)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.NoFileExists(t, path)
	require.NoError(t, os.Rename(intermediate, path))

	item.Title = "Edited"
	collision := itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item))
	sentinel := []byte("preserve unexpected content")
	require.NoError(t, os.WriteFile(collision, sentinel, 0600))
	require.ErrorContains(t, store.StoreItem(vault, item), "collision")
	unchanged, err := os.ReadFile(collision)
	require.NoError(t, err)
	require.Equal(t, sentinel, unchanged)
	current, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Original", current.Title)
}

func TestUncommittedNewItemIsCleanedAtUnlock(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Uncommitted", "synthetic")
	require.NoError(t, err)
	hash := testItemStateHash(t, item)
	root, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	require.NoError(t, root.MkdirAll(filepath.Dir(itemRelativePath(item.ID, hash)), 0700))
	require.NoError(t, store.storeEncryptedInRoot(root, itemRelativePath(item.ID, hash), vault.Key(), item))
	require.NoError(t, root.Close())
	loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	require.Empty(t, loaded.Accounts[0].Items)
	require.NoDirExists(t, filepath.Dir(itemPath(store, vault.Accounts[0].ID, item.ID, hash)))
}
