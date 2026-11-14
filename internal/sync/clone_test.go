// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"bytes"
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/test"
)

func TestBuildClonePlanCopiesOnlySourceItems(t *testing.T) {
	remote := &Manifest{Items: []ManifestItem{
		{ID: "first", ContentHash: hashByte(1), StateHash: hashByte(2)},
		{ID: "second", ContentHash: hashByte(3), StateHash: hashByte(4)},
	}}
	plan, err := BuildClonePlan(&Manifest{}, remote)
	require.NoError(t, err)
	require.Len(t, plan.Entries, 2)
	for _, entry := range plan.Entries {
		require.Equal(t, RemoteOnly, entry.Kind)
		require.Equal(t, CopyRemoteToLocal, entry.Decision)
	}
	ready, err := plan.Ready()
	require.NoError(t, err)
	require.True(t, ready)
	_, err = BuildClonePlan(remote, remote)
	require.ErrorContains(t, err, "must be empty")
}

func TestClonePairsBeforeVaultAndCopiesItems(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	sourceStore, sourceVault := pairingVault(t)
	defer sourceStore.Close()
	item, err := model.NewNote("Example", "contents")
	require.NoError(t, err)
	require.NoError(t, sourceStore.StoreItem(sourceVault, item))
	historicalFile, err := sourceStore.AddAttachment(sourceVault, item.ID, "old.txt", bytes.NewReader([]byte("historical attachment")))
	require.NoError(t, err)
	require.NoError(t, sourceStore.RemoveAttachment(sourceVault, item.ID, historicalFile.ID))
	currentFile, err := sourceStore.AddAttachment(sourceVault, item.ID, "current.txt", bytes.NewReader([]byte("current attachment")))
	require.NoError(t, err)
	before, err := sourceStore.LoadItem(sourceVault, item.ID)
	require.NoError(t, err)
	public, _, err := sourceStore.LoadOrCreateSyncIdentity(sourceVault)
	require.NoError(t, err)
	var sourceManager Manager
	source, err := sourceManager.StartManual(ctx, sourceStore, sourceVault, "Source")
	require.NoError(t, err)
	defer sourceManager.Stop()
	destStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer destStore.Close()
	var destManager Manager
	dest, err := destManager.StartClone(ctx, destStore, "Destination", false)
	require.NoError(t, err)
	defer destManager.Stop()
	peer := Peer{IP: net.ParseIP("127.0.0.1"), Port: source.Port(), Version: protocolVersion, Fingerprint: Fingerprint(public)}
	pairingResult := make(chan *Pairing, 1)
	pairingError := make(chan error, 1)
	go func() {
		pairing, err := dest.RequestPairing(ctx, peer)
		if err != nil {
			pairingError <- err
			return
		}
		pairingResult <- pairing
	}()
	var request PairingRequest
	select {
	case request = <-source.PairingRequests():
	case err := <-pairingError:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	pairing := awaitPairing(t, pairingResult)
	confirmed := make(chan error, 1)
	go func() { confirmed <- pairing.Confirm(ctx, pairing.Code()) }()
	require.NoError(t, request.Accept())
	require.NoError(t, awaitPairing(t, confirmed))
	require.NoError(t, request.Wait(ctx))
	vaults, err := destStore.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
	info, err := dest.RemoteVaultInfo(ctx)
	require.NoError(t, err)
	require.Equal(t, sourceVault.Accounts[0].ID, info.ID)
	remote, err := dest.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	require.Len(t, remote.Items, 1)
	destVault := test.CreateVault(t, destStore, info.Name)
	require.NotEqual(t, sourceVault.Accounts[0].ID, destVault.Accounts[0].ID)
	require.NotEqual(t, sourceVault.Key().String(), destVault.Key().String())
	require.NoError(t, dest.AttachCloneVault(destVault))
	local, err := BuildManifest(ctx, destStore, destVault)
	require.NoError(t, err)
	remote, err = dest.RemoteManifest(ctx, peer)
	require.NoError(t, err)
	plan, err := BuildClonePlan(local, remote)
	require.NoError(t, err)
	result, err := dest.Execute(ctx, peer, plan)
	require.NoError(t, err)
	require.Len(t, result.Completed, 1)
	cloned, err := destStore.LoadItem(destVault, item.ID)
	require.NoError(t, err)
	require.Equal(t, item.Title, cloned.Title)
	beforeHash, err := before.StateHash()
	require.NoError(t, err)
	clonedHash, err := cloned.StateHash()
	require.NoError(t, err)
	require.Equal(t, beforeHash, clonedHash)
	var attachment bytes.Buffer
	require.NoError(t, destStore.WriteAttachment(destVault, item.ID, historicalFile, &attachment))
	require.Equal(t, "historical attachment", attachment.String())
	attachment.Reset()
	require.NoError(t, destStore.WriteAttachment(destVault, item.ID, currentFile, &attachment))
	require.Equal(t, "current attachment", attachment.String())
	after, err := sourceStore.LoadItem(sourceVault, item.ID)
	require.NoError(t, err)
	afterHash, err := after.StateHash()
	require.NoError(t, err)
	require.Equal(t, beforeHash, afterHash)
	require.NoError(t, dest.NotifySyncComplete(ctx, peer, true))
	require.True(t, <-source.SyncOutcomes())
}

func TestFailedCloneRemovesSourcePairing(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	sourceStore, sourceVault := pairingVault(t)
	defer sourceStore.Close()
	public, _, err := sourceStore.LoadOrCreateSyncIdentity(sourceVault)
	require.NoError(t, err)
	var sourceManager Manager
	source, err := sourceManager.StartManual(ctx, sourceStore, sourceVault, "Source")
	require.NoError(t, err)
	defer sourceManager.Stop()
	destStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer destStore.Close()
	var destManager Manager
	dest, err := destManager.StartClone(ctx, destStore, "Destination", false)
	require.NoError(t, err)
	defer destManager.Stop()
	peer := Peer{IP: net.ParseIP("127.0.0.1"), Port: source.Port(), Version: protocolVersion, Fingerprint: Fingerprint(public)}
	pairings := make(chan *Pairing, 1)
	errors := make(chan error, 1)
	go func() {
		pairing, err := dest.RequestPairing(ctx, peer)
		if err != nil {
			errors <- err
			return
		}
		pairings <- pairing
	}()
	var request PairingRequest
	select {
	case request = <-source.PairingRequests():
	case err := <-errors:
		t.Fatal(err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	pairing := awaitPairing(t, pairings)
	confirmed := make(chan error, 1)
	go func() { confirmed <- pairing.Confirm(ctx, pairing.Code()) }()
	require.NoError(t, request.Accept())
	require.NoError(t, awaitPairing(t, confirmed))
	require.NoError(t, request.Wait(ctx))
	require.NoError(t, dest.NotifySyncComplete(ctx, peer, false))
	require.False(t, <-source.SyncOutcomes())
	peers, err := sourceStore.ListSyncPeers(sourceVault)
	require.NoError(t, err)
	require.Empty(t, peers)
}
