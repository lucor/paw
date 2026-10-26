// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/paw"
)

// TestLegacyReadersAuthenticateWholeFile checks valid JSON cannot hide a damaged age tail.
func TestLegacyReadersAuthenticateWholeFile(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	osBackend, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	osStorage := osBackend.(*OSStorage)
	fyneStorage, err := NewFyneStorage(rootedFyneStorage{
		Storage: app.Storage(), root: fynestorage.NewFileURI(osStorage.Root()),
	})
	require.NoError(t, err)
	fyneBackend := fyneStorage.(*FyneStorage)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault := paw.NewVault(key, "Personal")
	note := paw.NewNote()
	note.Name, note.Value = "Note", "Synthetic note"
	require.NoError(t, vault.AddItem(note))
	require.NoError(t, os.Mkdir(legacyVaultRootPath(osStorage, vault.Name), 0700))

	for _, file := range []struct {
		path  string
		value any
	}{
		{path: legacyVaultPath(osStorage, vault.Name), value: vault},
		{path: legacyItemPath(osStorage, vault.Name, note.ID()), value: note},
	} {
		data, err := json.Marshal(file.value)
		require.NoError(t, err)
		// Put the final authenticated age chunk after the JSON document.
		data = append(data, bytes.Repeat([]byte(" "), 128*1024)...)
		var ciphertext bytes.Buffer
		writeEncrypted(t, key, &ciphertext, data)
		require.NoError(t, os.WriteFile(file.path, ciphertext.Bytes()[:ciphertext.Len()-1], 0600))
	}
	for _, backend := range []struct {
		vault func() (*paw.Vault, error)
		item  func() (paw.Item, error)
	}{
		{vault: func() (*paw.Vault, error) { return osStorage.loadLegacyVault(vault.Name, key) }, item: func() (paw.Item, error) { return osStorage.loadLegacyItem(vault, note.Metadata) }},
		{vault: func() (*paw.Vault, error) { return fyneBackend.loadLegacyVault(vault.Name, key) }, item: func() (paw.Item, error) { return fyneBackend.loadLegacyItem(vault, note.Metadata) }},
	} {
		loadedVault, err := backend.vault()
		require.Error(t, err)
		require.Nil(t, loadedVault)
		loadedItem, err := backend.item()
		require.Error(t, err)
		require.Nil(t, loadedItem)
	}
}
