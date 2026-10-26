// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// TestNewOSStorageRootedDoesNotMoveUserData checks rooted storage stays isolated.
func TestNewOSStorageRootedDoesNotMoveUserData(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("HOME", configHome)
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("APPDATA", configHome)

	userConfigDir, err := os.UserConfigDir()
	require.NoError(t, err)
	legacyDir := filepath.Join(userConfigDir, "fyne", paw.ID, "vaults")
	require.NoError(t, os.MkdirAll(legacyDir, 0700))
	sentinel := filepath.Join(legacyDir, "keep")
	require.NoError(t, os.WriteFile(sentinel, []byte("test"), 0600))

	root := t.TempDir()
	_, err = NewOSStorageRooted(root)
	require.NoError(t, err)
	require.FileExists(t, sentinel)
}

// TestStorageOSRoundTrip checks the filesystem backend preserves encrypted vault data.
func TestStorageOSRoundTrip(t *testing.T) {
	name := "test"
	password := "secret"

	root, err := os.MkdirTemp(os.TempDir(), "paw")
	require.NoError(t, err)
	defer os.RemoveAll(root)

	storage, err := NewOSStorageRooted(root)
	require.NoError(t, err)

	vault, err := storage.CreateVault(name, password)
	require.NoError(t, err)
	require.Equal(t, name, vault.Metadata.VaultName)
	id := vault.Accounts[0].ID

	require.FileExists(t, vaultPath(storage, id))
	require.FileExists(t, keyPath(storage, id))

	note, err := model.NewNote("test note", "a secret note")
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, note))
	require.FileExists(t, itemPath(storage, id, note.ID))

	item, err := storage.LoadItem(vault, note.ID)
	require.NoError(t, err)
	require.Equal(t, note.Title, item.Title)

	login, err := model.NewLogin("test login", "test-user", "a secret password", model.PasswordSettings{})
	require.NoError(t, err)
	require.NoError(t, storage.StoreItem(vault, login))

	loadedVaultKey, err := storage.LoadVaultKey(id, password)
	require.NoError(t, err)

	loadedVault, err := storage.LoadVault(id, loadedVaultKey)
	require.NoError(t, err)
	require.Equal(t, name, loadedVault.Metadata.VaultName)
	require.Len(t, loadedVault.Accounts[0].Items, 2)

	itemWebsite, err := storage.LoadItem(loadedVault, login.ID)
	require.NoError(t, err)
	require.Equal(t, login.Title, itemWebsite.Title)
	require.NoError(t, storage.DeleteItem(loadedVault, note.ID))
	require.Len(t, loadedVault.Accounts[0].Items, 1)
	require.NoFileExists(t, itemPath(storage, id, note.ID))
}
