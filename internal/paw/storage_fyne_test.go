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

	app := test.NewApp()

	storage, err := NewFyneStorage(app.Storage())
	require.NoError(t, err)

	fs := storage.(*FyneStorage)
	vaultURI := fyneStorage.NewFileURI(vaultPath(fs, name))
	keyURI := fyneStorage.NewFileURI(keyPath(fs, name))

	defer func() {
		fyneStorage.Delete(vaultURI)
		fyneStorage.Delete(keyURI)
	}()

	// test key creation
	key, err := fs.CreateVaultKey(name, password)
	require.NoError(t, err)

	// test vault creation
	vault, err := fs.CreateVault(name, key)
	require.NoError(t, err)
	require.Equal(t, name, vault.Name)

	ok, err := fyneStorage.Exists(vaultURI)
	require.NoError(t, err)
	require.True(t, ok)

	ok, err = fyneStorage.Exists(keyURI)
	require.NoError(t, err)
	require.True(t, ok)

	// test item creation for the vault
	note := NewNote()
	note.Name = "test note"
	note.Value = "a secret note"

	vault.AddItem(note)
	// add note vault to item
	meta, ok := vault.ItemMetadata[note.Type][note.ID()]
	require.True(t, ok)
	assert.Equal(t, note.Name, meta.Name)

	// store note item
	err = fs.StoreItem(vault, note)
	require.NoError(t, err)

	itemURI := fyneStorage.NewFileURI(itemPath(fs, name, note.ID()))
	ok, err = fyneStorage.Exists(itemURI)
	require.NoError(t, err)
	require.True(t, ok)

	// test item load for the vault
	item, err := fs.LoadItem(vault, meta)
	require.NoError(t, err)
	require.NotNil(t, item)
	assert.Equal(t, note.Name, item.GetMetadata().Name)

	// test item creation for the vault
	login := NewLogin()
	login.Name = "test login"
	login.Password.Value = "a secret password"

	// add login item to vault
	vault.AddItem(login)
	require.Len(t, vault.ItemMetadata, 2) // login and note type

	err = fs.StoreItem(vault, login)
	require.NoError(t, err)

	loadedVault, err := fs.LoadVault(name, password)
	require.NoError(t, err)
	require.Equal(t, name, loadedVault.Name)
	require.Len(t, loadedVault.ItemMetadata, 2) // login and note type

	meta, ok = loadedVault.ItemMetadata[login.Type][login.ID()]
	require.True(t, ok)
	assert.Equal(t, login.Name, meta.Name)

	itemWebsite, err := fs.LoadItem(vault, meta)
	require.NoError(t, err)
	require.NotNil(t, itemWebsite)
	assert.Equal(t, login.Name, itemWebsite.GetMetadata().Name)
	assert.Equal(t, login.Password.Value, itemWebsite.(*Login).Password.Value)

}
