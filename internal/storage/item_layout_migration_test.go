// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

func TestCreateVaultCreatesItemStorageDirectory(t *testing.T) {
	store := newTestFileStorage(t)
	vault, err := store.CreateVault("Personal", "password")
	require.NoError(t, err)
	require.DirExists(t, filepath.Join(vaultRootPath(store, vault.Accounts[0].ID), itemsDirectoryName))
	loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	require.Empty(t, loaded.Accounts[0].Items)
}

func TestLoadVaultMigratesFlatItemFilesWithoutChangingContents(t *testing.T) {
	store, vault := newItemStoreVault(t)
	first := newMigrationTestItem(t, store, vault, "First")
	second := newMigrationTestItem(t, store, vault, "Second")
	firstHistory, err := first.History()
	require.NoError(t, err)
	require.NoError(t, firstHistory.AddSnapshot(first, 100))
	require.NoError(t, first.SetHistory(firstHistory))
	require.NoError(t, store.StoreItem(vault, first))
	firstContentHash, err := first.ContentHash()
	require.NoError(t, err)
	firstHash, err := first.StateHash()
	require.NoError(t, err)
	firstBytes := moveItemToLegacyPath(t, store, vault, first.ID)
	secondBytes := moveItemToLegacyPath(t, store, vault, second.ID)
	beforeSummaries := append([]cxf.Item(nil), vault.Accounts[0].Items...)
	keyBefore := []byte("key must remain untouched")
	require.NoError(t, os.WriteFile(keyPath(store, vault.Accounts[0].ID), keyBefore, 0o600))
	vaultBefore, err := os.ReadFile(vaultPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)
	metadataBefore := []byte("metadata must remain untouched")
	require.NoError(t, os.WriteFile(vaultMetadataPath(store, vault.Accounts[0].ID), metadataBefore, 0o600))

	loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	require.Equal(t, beforeSummaries, loaded.Accounts[0].Items)
	keyAfter, err := os.ReadFile(keyPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)
	vaultAfter, err := os.ReadFile(vaultPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)
	metadataAfter, err := os.ReadFile(vaultMetadataPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)
	require.Equal(t, keyBefore, keyAfter)
	require.Equal(t, vaultBefore, vaultAfter)
	require.Equal(t, metadataBefore, metadataAfter)
	require.NoFileExists(t, legacyItemPath(store, vault.Accounts[0].ID, first.ID))
	require.NoFileExists(t, legacyItemPath(store, vault.Accounts[0].ID, second.ID))
	firstMigrated, err := os.ReadFile(itemPath(store, vault.Accounts[0].ID, first.ID))
	require.NoError(t, err)
	secondMigrated, err := os.ReadFile(itemPath(store, vault.Accounts[0].ID, second.ID))
	require.NoError(t, err)
	require.Equal(t, firstBytes, firstMigrated)
	require.Equal(t, secondBytes, secondMigrated)
	loadedFirst, err := store.LoadItem(loaded, first.ID)
	require.NoError(t, err)
	afterHash, err := loadedFirst.StateHash()
	require.NoError(t, err)
	require.Equal(t, firstHash, afterHash)
	afterContentHash, err := loadedFirst.ContentHash()
	require.NoError(t, err)
	require.Equal(t, firstContentHash, afterContentHash)
	afterHistory, err := loadedFirst.History()
	require.NoError(t, err)
	require.Equal(t, firstHistory, afterHistory)

	// Loading another vault migrates only that vault.
	other, err := store.CreateVault("Other", "password")
	require.NoError(t, err)
	otherItem := newMigrationTestItem(t, store, other, "Other item")
	otherBytes := moveItemToLegacyPath(t, store, other, otherItem.ID)
	_, err = store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	stillLegacy, err := os.ReadFile(legacyItemPath(store, other.Accounts[0].ID, otherItem.ID))
	require.NoError(t, err)
	require.Equal(t, otherBytes, stillLegacy)
	require.NoFileExists(t, itemPath(store, other.Accounts[0].ID, otherItem.ID))
}

func TestLoadVaultResumesPartialItemLayoutMigration(t *testing.T) {
	store, vault := newItemStoreVault(t)
	first := newMigrationTestItem(t, store, vault, "First")
	second := newMigrationTestItem(t, store, vault, "Second")
	firstBytes := moveItemToLegacyPath(t, store, vault, first.ID)
	secondBytes := moveItemToLegacyPath(t, store, vault, second.ID)

	firstDir := filepath.Join(itemsDirectoryName, filesystemID(first.ID))
	root, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	require.NoError(t, root.MkdirAll(firstDir, 0o700))
	require.NoError(t, root.Rename(legacyItemPathRelative(first.ID), itemRelativePath(first.ID)))
	require.NoError(t, root.Close())

	loaded, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	firstAfter, err := os.ReadFile(itemPath(store, vault.Accounts[0].ID, first.ID))
	require.NoError(t, err)
	secondAfter, err := os.ReadFile(itemPath(store, vault.Accounts[0].ID, second.ID))
	require.NoError(t, err)
	require.Equal(t, firstBytes, firstAfter)
	require.Equal(t, secondBytes, secondAfter)
	require.NoError(t, store.migrateItemLayout(loaded))
	firstAfterRepeat, err := os.ReadFile(itemPath(store, vault.Accounts[0].ID, first.ID))
	require.NoError(t, err)
	secondAfterRepeat, err := os.ReadFile(itemPath(store, vault.Accounts[0].ID, second.ID))
	require.NoError(t, err)
	require.Equal(t, firstAfter, firstAfterRepeat)
	require.Equal(t, secondAfter, secondAfterRepeat)
}

func TestLoadVaultRejectsItemPathCollision(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item := newMigrationTestItem(t, store, vault, "Collision")
	legacyBytes := moveItemToLegacyPath(t, store, vault, item.ID)
	currentPath := itemPath(store, vault.Accounts[0].ID, item.ID)
	require.NoError(t, os.MkdirAll(filepath.Dir(currentPath), 0o700))
	currentBytes := []byte("current destination")
	require.NoError(t, os.WriteFile(currentPath, currentBytes, 0o600))

	_, err := store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.ErrorContains(t, err, "exists in both legacy and current storage paths")
	legacyAfter, err := os.ReadFile(legacyItemPath(store, vault.Accounts[0].ID, item.ID))
	require.NoError(t, err)
	currentAfter, err := os.ReadFile(currentPath)
	require.NoError(t, err)
	require.Equal(t, legacyBytes, legacyAfter)
	require.Equal(t, currentBytes, currentAfter)
}

func newMigrationTestItem(t *testing.T, store *FileStorage, vault *model.Vault, title string) *model.Item {
	t.Helper()
	item, err := model.NewNote(title, "synthetic migration content")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	return item
}

func moveItemToLegacyPath(t *testing.T, store *FileStorage, vault *model.Vault, itemID string) []byte {
	t.Helper()
	currentPath := itemPath(store, vault.Accounts[0].ID, itemID)
	data, err := os.ReadFile(currentPath)
	require.NoError(t, err)
	require.NoError(t, os.Rename(currentPath, legacyItemPath(store, vault.Accounts[0].ID, itemID)))
	return data
}

func legacyItemPath(store *FileStorage, vaultID, itemID string) string {
	return filepath.Join(vaultRootPath(store, vaultID), legacyItemPathRelative(itemID))
}

func legacyItemPathRelative(itemID string) string {
	return filesystemID(itemID) + ".age"
}
