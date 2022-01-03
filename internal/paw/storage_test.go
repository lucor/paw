package paw

import (
	"testing"

	fyneStorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageRoundTrip(t *testing.T) {
	name := "test"
	password := "secret"
	key, err := NewKey(name, password)
	require.NoError(t, err)

	app := test.NewApp()

	storage, err := NewStorage(app.Storage())
	require.NoError(t, err)
	defer fyneStorage.Delete(storage.vaultURI(name))

	// test vault creation
	vault, err := storage.CreateVault(key, name)
	require.NoError(t, err)
	require.Equal(t, name, vault.Name)

	ok, err := fyneStorage.Exists(storage.vaultURI(name))
	require.NoError(t, err)
	require.True(t, ok)

	// test item creation for the vault
	note := NewNote()
	note.Name = "test note"
	note.Value = "a secret note"

	vault.AddItem(note)
	// add note vault to item
	meta, ok := vault.ItemMetadata[note.ID()]
	require.True(t, ok)
	assert.Equal(t, note.Name, meta.Name)

	// store note item
	err = storage.StoreItem(vault, note)
	require.NoError(t, err)

	ok, err = fyneStorage.Exists(storage.itemURI(name, note.ID()))
	require.NoError(t, err)
	require.True(t, ok)

	// test item load for the vault
	item, err := storage.LoadItem(vault, meta)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, note.Name, item.GetMetadata().Name)

	// test item creation for the vault
	website := NewWebsite()
	website.Name = "test website"
	website.Password.Value = "a secret password"

	// add website item to vault
	vault.AddItem(website)
	require.Len(t, vault.ItemMetadata, 2) // website and note type

	err = storage.StoreItem(vault, website)
	require.NoError(t, err)

	loadedVault, err := storage.LoadVault(key, name)
	require.NoError(t, err)
	require.Equal(t, name, loadedVault.Name)
	require.Len(t, loadedVault.ItemMetadata, 2) // website and note type

	meta, ok = loadedVault.ItemMetadata[website.ID()]
	require.True(t, ok)
	assert.Equal(t, website.Name, meta.Name)

	itemWebsite, err := storage.LoadItem(vault, meta)
	require.NoError(t, err)
	require.NotNil(t, itemWebsite)
	assert.Equal(t, website.Name, itemWebsite.GetMetadata().Name)
	assert.Equal(t, website.Password.Value, itemWebsite.(*Website).Password.Value)

}
