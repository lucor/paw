// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageOSRoundTrip(t *testing.T) {
	name := "test"
	password := "secret"

	root, err := os.MkdirTemp(os.TempDir(), "paw")
	require.NoError(t, err)
	defer os.RemoveAll(root)

	storage, err := NewOSStorageRooted(root)
	require.NoError(t, err)

	vaultURI := vaultPath(storage, name)
	keyURI := keyPath(storage, name)

	// test key creation
	key, err := storage.CreateVaultKey(name, password)
	require.NoError(t, err)

	// test vault creation
	vault, err := storage.CreateVault(name, key)
	require.NoError(t, err)
	require.Equal(t, name, vault.Name)

	require.FileExists(t, vaultURI)
	require.FileExists(t, keyURI)

	// test item creation for the vault
	note := NewNote()
	note.Name = "test note"
	note.Value = "a secret note"

	err = vault.AddItem(note)
	require.NoError(t, err)
	// add note vault to item
	meta, ok := vault.ItemMetadata[note.Type][note.ID()]
	require.True(t, ok)
	assert.Equal(t, note.Name, meta.Name)

	// store note item
	err = storage.StoreItem(vault, note)
	require.NoError(t, err)

	itemURI := itemPath(storage, name, note.ID())
	require.FileExists(t, itemURI)

	// test item load for the vault
	item, err := storage.LoadItem(vault, meta)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, note.Name, item.GetMetadata().Name)

	// test item creation for the vault
	login := NewLogin()
	login.Name = "test login"
	login.Password.Value = "a secret password"

	// add login item to vault
	err = vault.AddItem(login)
	require.NoError(t, err)
	require.Len(t, vault.ItemMetadata, 2) // login and note type

	err = storage.StoreItem(vault, login)
	require.NoError(t, err)

	err = storage.StoreVault(vault)
	require.NoError(t, err)

	loadedVaultKey, err := storage.LoadVaultKey(name, password)
	require.NoError(t, err)

	loadedVault, err := storage.LoadVault(name, loadedVaultKey)
	require.NoError(t, err)
	require.Equal(t, name, loadedVault.Name)
	require.Len(t, loadedVault.ItemMetadata, 2) // login and note type

	meta, ok = loadedVault.ItemMetadata[login.Type][login.ID()]
	require.True(t, ok)
	assert.Equal(t, login.Name, meta.Name)

	itemWebsite, err := storage.LoadItem(vault, meta)
	require.NoError(t, err)
	require.NotNil(t, itemWebsite)
	assert.Equal(t, login.Name, itemWebsite.GetMetadata().Name)
	assert.Equal(t, login.Password.Value, itemWebsite.(*Login).Password.Value)
}
