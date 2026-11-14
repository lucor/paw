// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncCommandExposesReconcileFlag(t *testing.T) {
	command := newSyncCommand(&command{})
	require.Len(t, command.Flags, 3)
	require.Equal(t, "link", command.Flags[0].Names()[0])
	require.Equal(t, "clone", command.Flags[1].Names()[0])
	require.Equal(t, "reconcile", command.Flags[2].Names()[0])
}

func TestReconcileComparisonShowsOnlyDifferencesAndHidesOnlyPasswords(t *testing.T) {
	local, err := model.NewLogin("Access point", "same-user", "local-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	remote, err := model.NewLogin("Access point", "same-user", "remote-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, local.SetNote(-1, cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: "local note"}}))
	require.NoError(t, remote.SetNote(-1, cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: "remote note"}}))
	modified := uint64(1)
	localItem := reconcileItem{item: local, manifest: pawsync.ManifestItem{ModifiedAt: &modified}}
	remoteItem := reconcileItem{item: remote, manifest: pawsync.ManifestItem{ModifiedAt: &modified}}
	var output bytes.Buffer
	cmd := &SyncCmd{command: &command{out: &output}}

	require.NoError(t, cmd.showReconcileComparison(localItem, remoteItem, "fedora"))
	require.Contains(t, output.String(), "Password")
	require.Contains(t, output.String(), "[hidden]")
	require.Contains(t, output.String(), "local note")
	require.Contains(t, output.String(), "remote note")
	require.NotContains(t, output.String(), "local-secret")
	require.NotContains(t, output.String(), "remote-secret")
	require.NotContains(t, output.String(), "Username")
	require.NotContains(t, output.String(), "Modified")
	require.NotContains(t, output.String(), "History")
}

func TestReconcileContentHashIgnoresIDAndPreservesItems(t *testing.T) {
	first, err := model.NewNote("Example", "same content")
	require.NoError(t, err)
	second, err := model.NewNote("Example copy", "other content")
	require.NoError(t, err)
	secondCopy := *first
	secondCopy.ID = second.ID

	firstState, err := first.StateHash()
	require.NoError(t, err)
	secondState, err := secondCopy.StateHash()
	require.NoError(t, err)
	require.NotEqual(t, firstState, secondState)

	firstHash, err := contentHashReconcile(first)
	require.NoError(t, err)
	secondHash, err := contentHashReconcile(&secondCopy)
	require.NoError(t, err)
	require.Equal(t, firstHash, secondHash)

	firstStateAfter, err := first.StateHash()
	require.NoError(t, err)
	secondStateAfter, err := secondCopy.StateHash()
	require.NoError(t, err)
	require.Equal(t, firstState, firstStateAfter)
	require.Equal(t, secondState, secondStateAfter)
}

func TestReconcileContentHashDetectsDifferentContent(t *testing.T) {
	first, err := model.NewNote("Example", "first content")
	require.NoError(t, err)
	second, err := model.NewNote("Example", "second content")
	require.NoError(t, err)

	firstHash, err := contentHashReconcile(first)
	require.NoError(t, err)
	secondHash, err := contentHashReconcile(second)
	require.NoError(t, err)
	require.NotEqual(t, firstHash, secondHash)
}

func TestUniqueReconcileMatchesRejectsAmbiguousHashes(t *testing.T) {
	local, err := model.NewNote("Unique", "unique")
	require.NoError(t, err)
	remoteBase, err := model.NewNote("Unique copy", "other")
	require.NoError(t, err)
	remote := *local
	remote.ID = remoteBase.ID

	ambiguousA, err := model.NewNote("Duplicate", "duplicate")
	require.NoError(t, err)
	ambiguousBBase, err := model.NewNote("Duplicate copy", "other")
	require.NoError(t, err)
	ambiguousB := *ambiguousA
	ambiguousB.ID = ambiguousBBase.ID
	remoteAmbiguousBase, err := model.NewNote("Duplicate remote", "other")
	require.NoError(t, err)
	remoteAmbiguous := *ambiguousA
	remoteAmbiguous.ID = remoteAmbiguousBase.ID

	matches, err := uniqueReconcileMatches(
		[]*model.Item{local, ambiguousA, &ambiguousB},
		[]*model.Item{&remote, &remoteAmbiguous},
	)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, local.ID, matches[0].local.ID)
	require.Equal(t, remote.ID, matches[0].remote.ID)
}

func TestReconcileDifferentContentRequiresAnExplicitChoice(t *testing.T) {
	var output bytes.Buffer
	cmd := &SyncCmd{command: &command{out: &output}}
	input := make(chan syncInput, 1)
	input <- syncInput{line: "s"}

	decision, err := cmd.askReconcilePairDecision(context.Background(), input, "r", "Remote")
	require.NoError(t, err)
	require.Equal(t, "s", decision)
	require.Contains(t, output.String(), "Choice (suggested r)")
	require.Contains(t, output.String(), "Reject this match and keep both IDs")
}

func TestReconcileIsOnlyAFlagOnSync(t *testing.T) {
	syncCommand := newSyncCommand(&command{})
	require.Equal(t, "sync", syncCommand.Name)
	var found bool
	for _, flag := range syncCommand.Flags {
		if flag.Names()[0] == "reconcile" {
			found = true
		}
	}
	require.True(t, found)
}

func TestReconcileFlagAcceptsPositionAfterVaultArgument(t *testing.T) {
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	_, _, err = executeWithInput(
		t,
		[]string{"sync", "Personal", "--reconcile"},
		newSyncCommand(&command{
			storage: store,
			getenv:  func(string) string { return "" },
		}),
		"",
	)
	require.ErrorContains(t, err, `vault "Personal" does not exist`)
}

func TestReconcileOneSidedItemsAreCopiedWithoutRemoval(t *testing.T) {
	localItem, err := model.NewNote("Local only", "local")
	require.NoError(t, err)
	remoteItem, err := model.NewNote("Remote only", "remote")
	require.NoError(t, err)
	localHash, err := localItem.StateHash()
	require.NoError(t, err)
	remoteHash, err := remoteItem.StateHash()
	require.NoError(t, err)
	local := map[string]reconcileItem{
		localItem.ID: {item: localItem, manifest: pawsync.ManifestItem{ID: localItem.ID, Title: localItem.Title, Type: localItem.Metadata.Type, StateHash: localHash}},
	}
	remote := map[string]reconcileItem{
		remoteItem.ID: {item: remoteItem, manifest: pawsync.ManifestItem{ID: remoteItem.ID, Title: remoteItem.Title, Type: remoteItem.Metadata.Type, StateHash: remoteHash}},
	}

	operations := oneSidedReconcileOperations("Remote", local, remote, nil, nil)
	require.Len(t, operations, 2)
	require.True(t, operations[0].localSource)
	require.Equal(t, localItem.ID, operations[0].sourceID)
	require.Empty(t, operations[0].removeID)
	require.False(t, operations[1].localSource)
	require.Equal(t, remoteItem.ID, operations[1].sourceID)
	require.Empty(t, operations[1].removeID)
}

func TestReconcileHistoryEquivalenceIgnoresOwnerIDButNotUniqueEntries(t *testing.T) {
	local, err := model.NewNote("History", "same current content")
	require.NoError(t, err)
	remoteBase, err := model.NewNote("Remote History", "other")
	require.NoError(t, err)
	remote := *local
	remote.ID = remoteBase.ID

	localHistory := model.History{}
	require.NoError(t, localHistory.AddSnapshot(local, 10))
	require.NoError(t, local.SetHistory(localHistory))
	remoteHistory := model.History{}
	require.NoError(t, remoteHistory.AddSnapshot(&remote, 10))
	require.NoError(t, remote.SetHistory(remoteHistory))
	equivalent, err := reconcileHistoriesEquivalent(local, &remote)
	require.NoError(t, err)
	require.True(t, equivalent)

	require.NoError(t, remoteHistory.AddSnapshot(&remote, 20))
	require.NoError(t, remote.SetHistory(remoteHistory))
	equivalent, err = reconcileHistoriesEquivalent(local, &remote)
	require.NoError(t, err)
	require.False(t, equivalent)
}

type reconcileSessionStub struct {
	copyErr          error
	deleteLocalCalls int
	deletePeerCalls  int
	events           []string
}

func (stub *reconcileSessionStub) CopyItemToPeer(context.Context, pawsync.Peer, *model.Item, string, string) error {
	stub.events = append(stub.events, "copy")
	return stub.copyErr
}

func (stub *reconcileSessionStub) CopyRemoteItemToLocal(context.Context, pawsync.Peer, string, string, string) error {
	stub.events = append(stub.events, "copy-remote")
	return stub.copyErr
}

func (stub *reconcileSessionStub) DeleteLocalItem(context.Context, string, string) error {
	stub.deleteLocalCalls++
	stub.events = append(stub.events, "delete-local")
	return nil
}

func (stub *reconcileSessionStub) DeleteRemoteItem(context.Context, pawsync.Peer, string, string) error {
	stub.deletePeerCalls++
	stub.events = append(stub.events, "delete-peer")
	return nil
}

func TestReconcileTransferFailureNeverRemovesOldID(t *testing.T) {
	item, err := model.NewNote("Reconcile", "same content")
	require.NoError(t, err)
	var output, errOutput bytes.Buffer
	cmd := &SyncCmd{command: &command{out: &output, errOut: &errOutput}}
	transferErr := errors.New("transfer failed")
	stub := &reconcileSessionStub{copyErr: transferErr}
	operations := []reconcileOperation{{
		label:       "replace receiver duplicate",
		localSource: true,
		sourceID:    item.ID,
		removeID:    "old-id",
		removeState: "remote-hash",
	}}

	completed, err := cmd.executeReconciliation(context.Background(), stub, pawsync.Peer{Name: "Remote"}, map[string]reconcileItem{item.ID: {item: item}}, operations)
	require.ErrorIs(t, err, transferErr)
	require.False(t, completed)
	require.Zero(t, stub.deleteLocalCalls)
	require.Zero(t, stub.deletePeerCalls)
	require.Equal(t, []string{"copy"}, stub.events)
	require.Contains(t, errOutput.String(), "current transfer outcome is uncertain")
}

func TestReconcileDeletesOldIDOnlyAfterCopyVerification(t *testing.T) {
	item, err := model.NewNote("Reconcile", "same content")
	require.NoError(t, err)
	var output, errOutput bytes.Buffer
	cmd := &SyncCmd{command: &command{out: &output, errOut: &errOutput}}
	stub := &reconcileSessionStub{}
	operations := []reconcileOperation{{
		label:       "replace receiver duplicate",
		localSource: true,
		sourceID:    item.ID,
		removeID:    "old-id",
		removeState: "remote-hash",
	}}

	completed, err := cmd.executeReconciliation(context.Background(), stub, pawsync.Peer{Name: "Remote"}, map[string]reconcileItem{item.ID: {item: item}}, operations)
	require.NoError(t, err)
	require.True(t, completed)
	require.Equal(t, []string{"copy", "delete-peer"}, stub.events)
	require.Equal(t, 1, stub.deletePeerCalls)
	require.Contains(t, output.String(), "Copied and verified")
}

func TestNormalSyncStillMatchesOnlyByItemID(t *testing.T) {
	local, err := model.NewNote("Same", "logical item")
	require.NoError(t, err)
	remoteBase, err := model.NewNote("Same copy", "other")
	require.NoError(t, err)
	remote := *local
	remote.ID = remoteBase.ID
	localContent, err := local.ContentHash()
	require.NoError(t, err)
	remoteContent, err := remote.ContentHash()
	require.NoError(t, err)
	localState, err := local.StateHash()
	require.NoError(t, err)
	remoteState, err := remote.StateHash()
	require.NoError(t, err)
	diff, err := pawsync.BuildDiff(
		&pawsync.Manifest{Items: []pawsync.ManifestItem{{ID: local.ID, ContentHash: localContent, StateHash: localState}}},
		&pawsync.Manifest{Items: []pawsync.ManifestItem{{ID: remote.ID, ContentHash: remoteContent, StateHash: remoteState}}},
	)
	require.NoError(t, err)
	require.Len(t, diff, 2)
	kinds := map[pawsync.DifferenceKind]bool{diff[0].Kind: true, diff[1].Kind: true}
	require.True(t, kinds[pawsync.LocalOnly])
	require.True(t, kinds[pawsync.RemoteOnly])
}
