// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAtomicWriteReplacesBesideDestination(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vault.age"), []byte("old ciphertext"), 0600))

	err = atomicWrite(root, "vault.age", func(w io.Writer) error {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		for _, entry := range entries {
			if entry.Name() == "vault.age" {
				continue
			}
			info, err := entry.Info()
			require.NoError(t, err)
			if runtime.GOOS != "windows" {
				require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			}
			require.Equal(t, dir, filepath.Dir(filepath.Join(dir, entry.Name())))
		}
		_, err = w.Write([]byte("new ciphertext"))
		return err
	})
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "vault.age"))
	require.NoError(t, err)
	require.Equal(t, "new ciphertext", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"vault.age"}, []string{entries[0].Name()})
}

func TestAtomicWriteFailurePreservesDestinationAndCleansTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()
	destination := filepath.Join(dir, "paw.json")
	require.NoError(t, os.WriteFile(destination, []byte("previous settings"), 0600))

	writeErr := errors.New("serialization failed")
	err = atomicWrite(root, "paw.json", func(w io.Writer) error {
		if _, err := w.Write([]byte("partial settings")); err != nil {
			return err
		}
		return writeErr
	})
	require.ErrorIs(t, err, writeErr)
	data, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "previous settings", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"paw.json"}, []string{entries[0].Name()})
}

func TestAtomicWriteRenameFailurePreservesExistingPath(t *testing.T) {
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	defer root.Close()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "occupied"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "occupied", "sentinel"), []byte("keep"), 0600))

	err = atomicWrite(root, "occupied", func(w io.Writer) error {
		_, err := w.Write([]byte("replacement"))
		return err
	})
	require.Error(t, err)
	data, err := os.ReadFile(filepath.Join(dir, "occupied", "sentinel"))
	require.NoError(t, err)
	require.Equal(t, "keep", string(data))
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Equal(t, []string{"occupied"}, []string{entries[0].Name()})
}

func TestEncryptedWriteFailureKeepsPreviousCiphertext(t *testing.T) {
	store, vault := newItemStoreVault(t)
	path := vaultPath(store, vault.Accounts[0].ID)
	before, err := os.ReadFile(path)
	require.NoError(t, err)
	entriesBefore, err := os.ReadDir(vaultRootPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)

	vaultRoot, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, vaultRoot.Close()) })
	require.Error(t, store.storeEncryptedInRoot(vaultRoot, vaultFileName, vault.Key(), make(chan int)))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	entriesAfter, err := os.ReadDir(vaultRootPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)
	require.Len(t, entriesAfter, len(entriesBefore))
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
}

func TestFileStorageRootRejectsSymlinkEscape(t *testing.T) {
	rootPath := t.TempDir()
	externalPath := t.TempDir()
	sentinel := filepath.Join(externalPath, "sentinel")
	require.NoError(t, os.WriteFile(sentinel, []byte("keep"), 0600))
	store, err := NewFileStorage(rootPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	target, err := filepath.Rel(rootPath, externalPath)
	require.NoError(t, err)
	if err := store.root.Symlink(target, "escape"); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	if err := store.root.Symlink(filepath.Join(target, "sentinel"), appStateFileName); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err = store.LoadAppState()
	require.Error(t, err)
	data, err := os.ReadFile(sentinel)
	require.NoError(t, err)
	require.Equal(t, "keep", string(data))
}

func TestFileStorageRejectsVaultDirectorySymlink(t *testing.T) {
	store, vault := newItemStoreVault(t)
	id := vault.Accounts[0].ID
	require.NoError(t, store.root.RemoveAll(vaultDirectory(id)))
	linkPath := filepath.Join(storageRootName, filesystemID(id))
	if err := store.root.Symlink(".", linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := store.LoadVault(id, vault.Key())
	require.Error(t, err)
}
