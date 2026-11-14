// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/mldsa"
	"crypto/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/test"
)

func TestSyncIdentityPersistsPerVaultAndIsNotExported(t *testing.T) {
	store := newTestFileStorage(t)
	vault := test.CreateVaultWithPassword(t, store, "First", "password")
	publicKey, privateKey, err := store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	peerIdentity, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	require.NoError(t, store.AddSyncPeer(vault, SyncPeer{PublicKey: peerIdentity.PublicKey().Bytes(), Name: "Other device"}))
	require.Equal(t, mldsa.MLDSA44(), publicKey.Parameters())
	require.Len(t, publicKey.Bytes(), mldsa.MLDSA44PublicKeySize)
	require.Len(t, privateKey.Bytes(), mldsa.PrivateKeySize)
	require.True(t, privateKey.PublicKey().Equal(publicKey))

	message := []byte("Paw Sync identity")
	signature, err := privateKey.Sign(rand.Reader, message, nil)
	require.NoError(t, err)
	require.NoError(t, mldsa.Verify(publicKey, message, signature, nil))
	require.Error(t, mldsa.Verify(publicKey, []byte("different message"), signature, nil))

	path := filepath.Join(vaultRootPath(store, vault.Accounts[0].ID), syncIdentityFileName)
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	encrypted, err := os.ReadFile(path)
	require.NoError(t, err)
	privateKeyData := privateKey.Bytes()
	require.NotContains(t, string(encrypted), string(privateKeyData))
	clear(privateKeyData)
	decrypted, err := readDecrypted(vault.Key(), bytes.NewReader(encrypted))
	require.NoError(t, err)
	recovered, err := mldsa.NewPrivateKey(mldsa.MLDSA44(), decrypted)
	clear(decrypted)
	require.NoError(t, err)
	require.True(t, privateKey.Equal(recovered))

	store.Close()
	reopened, err := NewFileStorage(store.Root())
	require.NoError(t, err)
	defer reopened.Close()
	key, err := reopened.LoadVaultKey(vault.Accounts[0].ID, "password")
	require.NoError(t, err)
	loadedVault, err := reopened.LoadVault(vault.Accounts[0].ID, key)
	require.NoError(t, err)
	publicAgain, privateAgain, err := reopened.LoadOrCreateSyncIdentity(loadedVault)
	require.NoError(t, err)
	require.True(t, publicKey.Equal(publicAgain))
	require.True(t, privateKey.Equal(privateAgain))

	other := test.CreateVault(t, reopened, "Second")
	otherPublic, _, err := reopened.LoadOrCreateSyncIdentity(other)
	require.NoError(t, err)
	require.False(t, publicKey.Equal(otherPublic))

	var archive bytes.Buffer
	require.NoError(t, WritePawArchive(context.Background(), &archive, reopened, loadedVault, "archive-password"))
	backup, err := ReadPawArchive(context.Background(), bytes.NewReader(archive.Bytes()), "archive-password")
	require.NoError(t, err)
	defer backup.Close()
	reader := tar.NewReader(bytes.NewReader(backup.data))
	for {
		header, err := reader.Next()
		if err != nil {
			break
		}
		require.NotEqual(t, syncIdentityFileName, filepath.Base(header.Name))
		require.NotEqual(t, syncPeersFileName, filepath.Base(header.Name))
	}
}

func TestStoreSyncIdentityPreservesExistingIdentity(t *testing.T) {
	store := newTestFileStorage(t)
	vault := test.CreateVault(t, store, "Clone")
	identity, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	require.Error(t, store.StoreSyncIdentity(nil, identity))
	require.Error(t, store.StoreSyncIdentity(vault, nil))
	require.NoError(t, store.StoreSyncIdentity(vault, identity))
	public, loaded, err := store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	require.True(t, identity.Equal(loaded))
	require.True(t, identity.PublicKey().Equal(public))
	other, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	require.ErrorContains(t, store.StoreSyncIdentity(vault, other), "already exists")
	_, loaded, err = store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	require.True(t, identity.Equal(loaded))
}

func TestSyncIdentityRequiresUnlockedVaultAndRejectsCorruption(t *testing.T) {
	store := newTestFileStorage(t)
	vault := test.CreateVault(t, store, "Personal")
	_, _, err := store.LoadOrCreateSyncIdentity(nil)
	require.ErrorContains(t, err, "unlocked")

	path := filepath.Join(vaultRootPath(store, vault.Accounts[0].ID), syncIdentityFileName)
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0600))
	_, _, err = store.LoadOrCreateSyncIdentity(vault)
	require.Error(t, err)
	require.Equal(t, []byte("corrupt"), mustReadFile(t, path))

	var invalidIdentity bytes.Buffer
	writer, err := vault.Key().Encrypt(&invalidIdentity)
	require.NoError(t, err)
	_, err = writer.Write([]byte("invalid key"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	require.NoError(t, os.WriteFile(path, invalidIdentity.Bytes(), 0600))
	storedIdentity := mustReadFile(t, path)
	_, _, err = store.LoadOrCreateSyncIdentity(vault)
	require.ErrorContains(t, err, "invalid Sync identity")
	require.Equal(t, storedIdentity, mustReadFile(t, path))

	wrongKey := test.CreateVault(t, store, "Other")
	_, _, err = store.LoadOrCreateSyncIdentity(wrongKey)
	require.NoError(t, err)
}

func TestDeleteVaultRemovesSyncIdentity(t *testing.T) {
	store := newTestFileStorage(t)
	vault := test.CreateVault(t, store, "Personal")
	_, _, err := store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	require.NoError(t, store.DeleteVault(vault.Accounts[0].ID))
	require.NoDirExists(t, vaultRootPath(store, vault.Accounts[0].ID))
}

func mustReadFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
