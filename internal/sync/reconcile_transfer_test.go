// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestReconcileTransfersHistoryAndAttachmentsBeforeRemoval(t *testing.T) {
	ctx := context.Background()
	localStore, local := pairingVault(t)
	defer localStore.Close()
	var exported bytes.Buffer
	require.NoError(t, storage.WriteCXF(ctx, &exported, localStore, local))
	backup, err := storage.ReadPawBackup(ctx, bytes.NewReader(exported.Bytes()), "")
	require.NoError(t, err)
	defer backup.Close()
	remoteStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer remoteStore.Close()
	remote, err := remoteStore.Restore(ctx, backup, "Remote", "different-password")
	require.NoError(t, err)
	localPublic, localKey, err := localStore.LoadOrCreateSyncIdentity(local)
	require.NoError(t, err)
	remotePublic, remoteKey, err := remoteStore.LoadOrCreateSyncIdentity(remote)
	require.NoError(t, err)
	require.NoError(t, localStore.AddSyncPeer(local, storage.SyncPeer{
		PublicKey: remotePublic.Bytes(),
		Name:      "Remote",
	}))
	require.NoError(t, remoteStore.AddSyncPeer(remote, storage.SyncPeer{
		PublicKey: localPublic.Bytes(),
		Name:      "Local",
	}))
	address, _ := startPairingEndpoint(t, remoteStore, remote, remotePublic, remoteKey, "Remote")
	client, err := NewClient(localKey)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	session := &Session{ctx: ctx, client: client, store: localStore, vault: local, public: localPublic}
	peer := Peer{
		IP:          net.ParseIP("127.0.0.1"),
		Port:        address.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(remotePublic),
	}

	localItem, err := model.NewNote("Planner item", "same logical content")
	require.NoError(t, err)
	require.NoError(t, localStore.StoreItem(local, localItem))
	oldAttachment, err := localStore.AddAttachment(local, localItem.ID, "old.txt", bytes.NewReader([]byte("historical attachment")))
	require.NoError(t, err)
	require.NoError(t, localStore.RemoveAttachment(local, localItem.ID, oldAttachment.ID))
	currentAttachment, err := localStore.AddAttachment(local, localItem.ID, "current.txt", bytes.NewReader([]byte("current attachment")))
	require.NoError(t, err)
	localItem, err = localStore.LoadItem(local, localItem.ID)
	require.NoError(t, err)
	localHistory, err := localItem.History()
	require.NoError(t, err)
	require.NoError(t, localHistory.AddSnapshot(localItem, 40))
	require.NoError(t, localItem.SetHistory(localHistory))
	require.NoError(t, localStore.StoreItem(local, localItem))
	localItem, err = localStore.LoadItem(local, localItem.ID)
	require.NoError(t, err)
	localState, err := localItem.StateHash()
	require.NoError(t, err)
	require.NoError(t, session.CopyItemToPeer(ctx, peer, localItem, localState, ""))
	remoteCopy, err := remoteStore.LoadItem(remote, localItem.ID)
	require.NoError(t, err)
	remoteState, err := remoteCopy.StateHash()
	require.NoError(t, err)
	require.Equal(t, localState, remoteState)
	remoteHistory, err := remoteCopy.History()
	require.NoError(t, err)
	require.Equal(t, localHistory, remoteHistory)
	for _, file := range []cxf.File{oldAttachment, currentAttachment} {
		valid, err := remoteStore.VerifyAttachment(remote, localItem.ID, file)
		require.NoError(t, err)
		require.True(t, valid)
	}
	require.NoError(t, session.DeleteRemoteItem(ctx, peer, localItem.ID, localState))
	_, err = remoteStore.LoadItem(remote, localItem.ID)
	require.Error(t, err)

	remoteItem, err := model.NewNote("Responder item", "remote content")
	require.NoError(t, err)
	require.NoError(t, remoteStore.StoreItem(remote, remoteItem))
	remoteAttachment, err := remoteStore.AddAttachment(remote, remoteItem.ID, "remote.txt", bytes.NewReader([]byte("remote attachment")))
	require.NoError(t, err)
	remoteItem, err = remoteStore.LoadItem(remote, remoteItem.ID)
	require.NoError(t, err)
	remoteItemHistory, err := remoteItem.History()
	require.NoError(t, err)
	require.NoError(t, remoteItemHistory.AddSnapshot(remoteItem, 50))
	require.NoError(t, remoteItem.SetHistory(remoteItemHistory))
	require.NoError(t, remoteStore.StoreItem(remote, remoteItem))
	remoteItem, err = remoteStore.LoadItem(remote, remoteItem.ID)
	require.NoError(t, err)
	remoteItemState, err := remoteItem.StateHash()
	require.NoError(t, err)
	require.NoError(t, session.CopyRemoteItemToLocal(ctx, peer, remoteItem.ID, remoteItemState, ""))
	localCopy, err := localStore.LoadItem(local, remoteItem.ID)
	require.NoError(t, err)
	localCopyHistory, err := localCopy.History()
	require.NoError(t, err)
	require.Equal(t, remoteItemHistory, localCopyHistory)
	valid, err := localStore.VerifyAttachment(local, remoteItem.ID, remoteAttachment)
	require.NoError(t, err)
	require.True(t, valid)
	require.NoError(t, session.DeleteLocalItem(ctx, remoteItem.ID, remoteItemState))
}
