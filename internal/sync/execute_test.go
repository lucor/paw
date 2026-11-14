// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

type mutatingAttachmentStore struct {
	*storage.FileStorage
	vault  *model.Vault
	itemID string
}

func (s *mutatingAttachmentStore) ReceiveAttachment(vault *model.Vault, itemID string, file cxf.File, src io.Reader) (bool, error) {
	created, err := s.FileStorage.ReceiveAttachment(vault, itemID, file, src)
	if err != nil || !created || itemID != s.itemID {
		return created, err
	}
	item, err := s.FileStorage.LoadItem(s.vault, s.itemID)
	if err != nil {
		return false, err
	}
	item.Title = "changed while attachment transferred"
	return created, s.FileStorage.StoreItem(s.vault, item)
}

// TestExecuteCopiesAndDeletes uses real storage and mutual TLS without discovery.
func TestExecuteCopiesAndDeletes(t *testing.T) {
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
	require.Equal(t, local.Accounts[0].ID, remote.Accounts[0].ID)
	require.NotEqual(t, local.Key().String(), remote.Key().String())
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
	endpointStore := &mutatingAttachmentStore{
		FileStorage: remoteStore,
		vault:       remote,
	}
	address, _, receiverProgress := startPairingEndpointWithProgress(
		t,
		remoteStore,
		endpointStore,
		remote,
		remotePublic,
		remoteKey,
		"Remote",
	)
	client, err := NewClient(localKey)
	require.NoError(t, err)
	defer client.CloseIdleConnections()
	session := &Session{
		ctx:    ctx,
		client: client,
		store:  localStore,
		vault:  local,
		public: localPublic,
	}
	peer := Peer{
		IP:          net.ParseIP("127.0.0.1"),
		Port:        address.Port,
		Version:     protocolVersion,
		Fingerprint: Fingerprint(remotePublic),
	}
	first, err := model.NewNote("Local", "local state")
	require.NoError(t, err)
	second, err := model.NewNote("Remote", "remote state")
	require.NoError(t, err)
	require.NoError(t, localStore.StoreItem(local, first))
	require.NoError(t, remoteStore.StoreItem(remote, second))
	firstFile, err := localStore.AddAttachment(local, first.ID, "local.txt", bytes.NewReader([]byte("local attachment")))
	require.NoError(t, err)
	require.NoError(t, localStore.RemoveAttachment(local, first.ID, firstFile.ID))
	localCurrentFile, err := localStore.AddAttachment(
		local,
		first.ID,
		"current.txt",
		bytes.NewReader([]byte("current attachment")),
	)
	require.NoError(t, err)
	secondFile, err := remoteStore.AddAttachment(remote, second.ID, "remote.txt", bytes.NewReader([]byte("remote attachment")))
	require.NoError(t, err)
	unavailable, err := model.NewNote("Unavailable", "missing blob")
	require.NoError(t, err)
	require.NoError(t, unavailable.AddFile(firstFile))
	missingHash, err := unavailable.StateHash()
	require.NoError(t, err)
	remotePeer := newAuthenticatedPeer(client, address.String(), remotePublic)
	_, err = remotePeer.transferItem(ctx, http.MethodPut, itemRequest{
		ID:       unavailable.ID,
		Expected: "",
		Source:   missingHash,
		Item:     unavailable,
	})
	require.Error(t, err)
	_, err = remoteStore.LoadItem(remote, unavailable.ID)
	require.Error(t, err)
	staleTarget, err := model.NewNote("Stale target", "before transfer")
	require.NoError(t, err)
	require.NoError(t, remoteStore.StoreItem(remote, staleTarget))
	preserved, err := remoteStore.LoadItem(remote, staleTarget.ID)
	require.NoError(t, err)
	raceHash, err := preserved.StateHash()
	require.NoError(t, err)
	source, err := model.NewNote("Stale target", "source item")
	require.NoError(t, err)
	source.ID = staleTarget.ID
	require.NoError(t, source.AddFile(firstFile))
	_, err = localStore.ReceiveAttachment(local, source.ID, firstFile, bytes.NewReader([]byte("local attachment")))
	require.NoError(t, err)
	sourceHash, err := source.StateHash()
	require.NoError(t, err)
	endpointStore.itemID = staleTarget.ID
	_, err = remotePeer.transferAttachment(ctx, firstFile, source.ID, bytes.NewReader([]byte("local attachment")))
	require.NoError(t, err)
	_, err = remotePeer.transferItem(ctx, http.MethodPut, itemRequest{
		ID:       source.ID,
		Expected: raceHash,
		Source:   sourceHash,
		Item:     source,
	})
	require.Error(t, err)
	preserved, err = remoteStore.LoadItem(remote, staleTarget.ID)
	require.NoError(t, err)
	require.Equal(t, "changed while attachment transferred", preserved.Title)
	require.NoError(t, remoteStore.DeleteItem(remote, staleTarget.ID))
	localManifest, err := BuildManifest(ctx, localStore, local)
	require.NoError(t, err)
	remoteManifest, err := session.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	diff, err := BuildDiff(localManifest, remoteManifest)
	require.NoError(t, err)
	plan, err := BuildPlan(diff, localManifest, remoteManifest)
	require.NoError(t, err)
	var progress []struct {
		entry   PlanEntry
		started bool
		err     error
	}
	result, err := session.ExecuteWithProgress(ctx, peer, plan, func(entry PlanEntry, started bool, err error) {
		progress = append(progress, struct {
			entry   PlanEntry
			started bool
			err     error
		}{entry: entry, started: started, err: err})
	})
	require.NoError(t, err)
	require.Len(t, result.Completed, 2)
	require.Nil(t, result.Failed)
	require.Empty(t, result.Unattempted)
	require.Len(t, progress, 4)
	for index := 0; index < len(progress); index += 2 {
		require.True(t, progress[index].started)
		require.NoError(t, progress[index].err)
		require.False(t, progress[index+1].started)
		require.NoError(t, progress[index+1].err)
		require.Equal(t, progress[index].entry, progress[index+1].entry)
	}
	var received []SyncProgressEvent
	for len(received) < 2 {
		received = append(received, <-receiverProgress)
	}
	require.Equal(t, first.ID, received[0].ItemID)
	require.Equal(t, "Local", received[0].Title)
	require.True(t, received[0].Started)
	require.Equal(t, first.ID, received[1].ItemID)
	require.True(t, received[1].Completed)
	for _, original := range []*model.Item{first, second} {
		localItem, err := localStore.LoadItem(local, original.ID)
		require.NoError(t, err)
		remoteItem, err := remoteStore.LoadItem(remote, original.ID)
		require.NoError(t, err)
		localHash, err := localItem.StateHash()
		require.NoError(t, err)
		remoteHash, err := remoteItem.StateHash()
		require.NoError(t, err)
		require.Equal(t, localHash, remoteHash)
	}
	var copied bytes.Buffer
	require.NoError(t, remoteStore.WriteAttachment(remote, first.ID, firstFile, &copied))
	require.Equal(t, []byte("local attachment"), copied.Bytes())
	copied.Reset()
	require.NoError(t, remoteStore.WriteAttachment(remote, first.ID, localCurrentFile, &copied))
	require.Equal(t, []byte("current attachment"), copied.Bytes())
	copied.Reset()
	require.NoError(t, localStore.WriteAttachment(local, second.ID, secondFile, &copied))
	require.Equal(t, []byte("remote attachment"), copied.Bytes())
	require.NoError(t, remotePeer.removeRemoteAttachment(
		ctx,
		localCurrentFile,
		first.ID,
	))
	valid, err := remoteStore.VerifyAttachment(remote, first.ID, localCurrentFile)
	require.NoError(t, err)
	require.True(t, valid)
	orphanItem, err := model.NewNote("Orphan", "uncommitted attachment")
	require.NoError(t, err)
	created, err := remoteStore.ReceiveAttachment(remote, orphanItem.ID, firstFile, bytes.NewReader([]byte("local attachment")))
	require.NoError(t, err)
	require.True(t, created)
	require.NoError(t, remotePeer.removeRemoteAttachment(ctx, firstFile, orphanItem.ID))
	valid, err = remoteStore.VerifyAttachment(remote, orphanItem.ID, firstFile)
	require.NoError(t, err)
	require.False(t, valid)
	// Reusing the original plan must fail rather than overwrite changed presence.
	result, err = session.Execute(ctx, peer, plan)
	require.ErrorContains(t, err, "rebuild")
	require.Empty(t, result.Completed)
	require.NotNil(t, result.Failed)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = session.Execute(canceled, peer, plan)
	require.Error(t, err)

	// A replacement carries the source history and state hash to the remote vault.
	current, err := localStore.LoadItem(local, first.ID)
	require.NoError(t, err)
	history, err := current.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(current, 1))
	current.Title = "Updated"
	require.NoError(t, current.SetHistory(history))
	require.NoError(t, localStore.StoreItem(local, current))
	localManifest, err = BuildManifest(ctx, localStore, local)
	require.NoError(t, err)
	remoteManifest, err = session.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	diff, err = BuildDiff(localManifest, remoteManifest)
	require.NoError(t, err)
	plan, err = BuildPlan(diff, localManifest, remoteManifest)
	require.NoError(t, err)
	require.NoError(t, plan.SetDecision(first.ID, CopyLocalToRemote))
	_, err = session.Execute(ctx, peer, plan)
	require.NoError(t, err)
	remoteItem, err := remoteStore.LoadItem(remote, first.ID)
	require.NoError(t, err)
	expectedHash, err := current.StateHash()
	require.NoError(t, err)
	actualHash, err := remoteItem.StateHash()
	require.NoError(t, err)
	require.Equal(t, expectedHash, actualHash)
	remoteHistory, err := remoteItem.History()
	require.NoError(t, err)
	require.Equal(t, history, remoteHistory)

	// Missing items are explicit decisions; exercise deletion on both devices.
	require.NoError(t, remoteStore.DeleteItem(remote, first.ID))
	require.NoError(t, localStore.DeleteItem(local, second.ID))
	localManifest, err = BuildManifest(ctx, localStore, local)
	require.NoError(t, err)
	remoteManifest, err = session.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	diff, err = BuildDiff(localManifest, remoteManifest)
	require.NoError(t, err)
	plan, err = BuildPlan(diff, localManifest, remoteManifest)
	require.NoError(t, err)
	require.NoError(t, plan.SetDecision(first.ID, DeleteLocal))
	require.NoError(t, plan.SetDecision(second.ID, DeleteRemote))
	result, err = session.Execute(ctx, peer, plan)
	require.NoError(t, err)
	require.Len(t, result.Completed, 2)
	_, err = localStore.LoadItem(local, first.ID)
	require.Error(t, err)
	_, err = remoteStore.LoadItem(remote, second.ID)
	require.Error(t, err)

	// Invalidate the second entry after planning and preserve the first commit.
	require.NoError(t, localStore.StoreItem(local, first))
	require.NoError(t, localStore.StoreItem(local, second))
	localManifest, err = BuildManifest(ctx, localStore, local)
	require.NoError(t, err)
	remoteManifest, err = session.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	diff, err = BuildDiff(localManifest, remoteManifest)
	require.NoError(t, err)
	plan, err = BuildPlan(diff, localManifest, remoteManifest)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 2)
	stale, err := localStore.LoadItem(local, plan.Entries[1].ItemID)
	require.NoError(t, err)
	stale.Title = "Changed after planning"
	require.NoError(t, localStore.StoreItem(local, stale))
	result, err = session.Execute(ctx, peer, plan)
	require.ErrorContains(t, err, "rebuild")
	require.Len(t, result.Completed, 1)
	require.Equal(t, plan.Entries[1].ItemID, result.Failed.ItemID)
	require.Empty(t, result.Unattempted)
	_, err = remoteStore.LoadItem(remote, plan.Entries[0].ItemID)
	require.NoError(t, err)

	// The authenticated caller loses access when the responder removes trust.
	require.NoError(t, remoteStore.RemoveSyncPeer(remote, localPublic.Bytes()))
	_, err = session.Execute(ctx, peer, plan)
	require.Error(t, err)
}
