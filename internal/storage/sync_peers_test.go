// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"crypto/mldsa"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/test"
)

func TestSyncPeersPersistEncryptedAndCanBeRemoved(t *testing.T) {
	store := newTestFileStorage(t)
	vault := test.CreateVaultWithPassword(t, store, "Personal", "password")
	private, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	public := private.PublicKey()
	peer := SyncPeer{
		PublicKey: public.Bytes(),
		Name:      "Other device",
	}
	require.NoError(t, store.AddSyncPeer(vault, peer))
	require.NoError(t, store.AddSyncPeer(vault, peer))
	peers, err := store.ListSyncPeers(vault)
	require.NoError(t, err)
	require.Equal(t, []SyncPeer{peer}, peers)

	path := filepath.Join(vaultRootPath(store, vault.Accounts[0].ID), syncPeersFileName)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotContains(t, string(data), string(peer.PublicKey))
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}

	store.Close()
	reopened, err := NewFileStorage(store.Root())
	require.NoError(t, err)
	defer reopened.Close()
	key, err := reopened.LoadVaultKey(vault.Accounts[0].ID, "password")
	require.NoError(t, err)
	loaded, err := reopened.LoadVault(vault.Accounts[0].ID, key)
	require.NoError(t, err)
	peers, err = reopened.ListSyncPeers(loaded)
	require.NoError(t, err)
	require.Equal(t, []SyncPeer{peer}, peers)
	require.NoError(t, reopened.RemoveSyncPeer(loaded, peer.PublicKey))
	peers, err = reopened.ListSyncPeers(loaded)
	require.NoError(t, err)
	require.Empty(t, peers)
}

func TestSyncPeersReadFailureDoesNotOverwriteTrust(t *testing.T) {
	store := newTestFileStorage(t)
	vault := test.CreateVault(t, store, "Personal")
	path := filepath.Join(vaultRootPath(store, vault.Accounts[0].ID), syncPeersFileName)
	require.NoError(t, os.WriteFile(path, []byte("corrupt"), 0600))
	_, err := store.ListSyncPeers(vault)
	require.Error(t, err)
	require.Equal(t, []byte("corrupt"), mustReadFile(t, path))
	private, err := mldsa.GenerateKey(mldsa.MLDSA44())
	require.NoError(t, err)
	public := private.PublicKey()
	require.Error(t, store.AddSyncPeer(vault, SyncPeer{
		PublicKey: public.Bytes(),
		Name:      "device",
	}))
	require.Equal(t, []byte("corrupt"), mustReadFile(t, path))
}
