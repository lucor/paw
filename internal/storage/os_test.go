// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/model"
)

// TestFileStorageRoundTrip checks the filesystem backend preserves encrypted vault data.
func TestFileStorageRoundTrip(t *testing.T) {
	name := "test"
	password := "secret"
	storage := newTestFileStorage(t)
	appearance := model.VaultAppearance{Icon: "laptop", Color: "purple"}
	vault, err := storage.CreateVault(name, password, appearance)
	require.NoError(t, err)
	require.Equal(t, name, vault.Metadata.VaultName)
	id := vault.Accounts[0].ID
	reopened, err := NewFileStorage(storage.Root())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })

	require.FileExists(t, vaultPath(storage, id))
	require.FileExists(t, keyPath(storage, id))
	require.FileExists(t, vaultMetadataPath(storage, id))
	directoryID := filepath.Base(vaultRootPath(storage, id))
	require.NotContains(t, directoryID, "-")
	require.Equal(t, strings.ToLower(directoryID), directoryID)
	require.True(t, isFilesystemVaultID(directoryID))

	note, err := model.NewNote("test note", "a secret note")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, note))
	require.FileExists(t, itemPath(storage, id, note.ID, testItemStateHash(t, note)))

	item, err := reopened.LoadItem(vault, note.ID)
	require.NoError(t, err)
	require.Equal(t, note.Title, item.Title)

	login, err := model.NewLogin("test login", "test-user", "a secret password", model.PasswordSettings{})
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, login))
	state := &model.AppState{Modified: time.Now().UTC(), Settings: model.NewDefaultSettings()}
	state.Settings.FaviconDownloader.Disabled = true
	state.Settings.Password.Random.DefaultLength = 24
	require.NoError(t, storage.StoreAppState(state))
	loadedState, err := reopened.LoadAppState()
	require.NoError(t, err)
	require.Equal(t, state, loadedState)
	vaults, err := reopened.Vaults()
	require.NoError(t, err)
	require.Contains(t, vaults, model.VaultInfo{ID: id, Name: name, Appearance: appearance})

	loadedVault, err := reopened.LoadVault(id, vault.Key())
	require.NoError(t, err)
	require.Equal(t, name, loadedVault.Metadata.VaultName)
	require.Len(t, loadedVault.Accounts[0].Items, 2)
	keyString := vault.Key().String()
	require.NoError(t, reopened.ChangeMasterPassword(context.Background(), id, password, "new secret"))
	newKey, err := reopened.LoadVaultKey(id, "new secret")
	require.NoError(t, err)
	require.Equal(t, keyString, newKey.String())
	loadedVault, err = reopened.LoadVault(id, newKey)
	require.NoError(t, err)

	itemWebsite, err := reopened.LoadItem(loadedVault, login.ID)
	require.NoError(t, err)
	require.Equal(t, login.Title, itemWebsite.Title)
	require.NoError(t, reopened.DeleteItem(loadedVault, note.ID))
	require.Len(t, loadedVault.Accounts[0].Items, 1)
	require.NoFileExists(t, itemPath(storage, id, note.ID, testItemStateHash(t, note)))
	require.NoError(t, reopened.DeleteVault(id))
	require.NoDirExists(t, vaultRootPath(storage, id))
}
