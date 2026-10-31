// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestChangeMasterPasswordCancellationBeforePublication(t *testing.T) {
	store, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	vault, err := store.CreateVault("Personal", "old password")
	require.NoError(t, err)
	id := vault.Accounts[0].ID
	path := keyPath(store, id)
	original, err := os.ReadFile(path)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- store.ChangeMasterPassword(ctx, id, "old password", "new password") }()
	pattern := filepath.Join(filepath.Dir(path), ".key-*")
	require.Eventually(t, func() bool { files, _ := filepath.Glob(pattern); return len(files) > 0 }, 10*time.Second, time.Millisecond)
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, data)
	files, err := filepath.Glob(pattern)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestChangeMasterPasswordPreservesVaultAndItems(t *testing.T) {
	for _, name := range []string{"OS", "Fyne"} {
		t.Run(name, func(t *testing.T) {
			app := test.NewTempApp(t)
			base, err := NewOSStorageRooted(t.TempDir())
			require.NoError(t, err)
			var store Storage = base
			if name == "Fyne" {
				store, err = NewFyneStorage(rootedFyneStorage{Storage: app.Storage(), root: fynestorage.NewFileURI(base.Root())})
				require.NoError(t, err)
			}
			vault, err := store.CreateVault("Personal", "old password")
			require.NoError(t, err)
			item, err := model.NewNote("Note", "Synthetic note")
			require.NoError(t, err)
			require.NoError(t, store.StoreItem(vault, item))
			id := vault.Accounts[0].ID
			paths := []string{vaultPath(base, id), itemPath(base, id, item.ID), vaultMetadataPath(base, id)}
			before := make([][]byte, len(paths))
			for i, path := range paths {
				before[i], err = os.ReadFile(path)
				require.NoError(t, err)
			}
			original, err := os.ReadFile(keyPath(base, id))
			require.NoError(t, err)
			require.ErrorIs(t, store.ChangeMasterPassword(context.Background(), id, "wrong", "new password"), ErrIncorrectMasterPassword)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			require.ErrorIs(t, store.ChangeMasterPassword(ctx, id, "old password", "new password"), context.Canceled)
			require.Error(t, store.ChangeMasterPassword(context.Background(), id, "old password", ""))
			unchanged, err := os.ReadFile(keyPath(base, id))
			require.NoError(t, err)
			require.Equal(t, original, unchanged)
			require.NoError(t, store.ChangeMasterPassword(context.Background(), id, "old password", "new password"))
			_, err = store.LoadVaultKey(id, "old password")
			require.Error(t, err)
			key, err := store.LoadVaultKey(id, "new password")
			require.NoError(t, err)
			require.Equal(t, vault.Key().String(), key.String())
			loaded, err := store.LoadVault(id, key)
			require.NoError(t, err)
			_, err = store.LoadItem(loaded, item.ID)
			require.NoError(t, err)
			for i, path := range paths {
				data, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, before[i], data)
			}
			info, err := os.Stat(keyPath(base, id))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(keyPath(base, id)), ".key-*"))
			require.NoError(t, err)
			require.Empty(t, leftovers)
		})
	}
}
