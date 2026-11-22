// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/test"
	"rsc.io/qr"
)

func TestSyncLinkActionsKeepIncomingPairingAvailable(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	vault := test.CreateVault(t, store, "Receiver")
	var receiverManager pawsync.Manager
	receiver, err := receiverManager.StartManual(ctx, store, vault, "Receiver")
	require.NoError(t, err)
	defer receiverManager.Stop()
	public, _, err := store.LoadOrCreateSyncIdentity(vault)
	require.NoError(t, err)
	link := pawsync.ConnectionLink{
		Host:        "127.0.0.1",
		Port:        receiver.Port(),
		Fingerprint: pawsync.Fingerprint(public),
	}

	otherStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer otherStore.Close()
	otherVault := test.CreateVault(t, otherStore, "Sender")
	var senderManager pawsync.Manager
	sender, err := senderManager.StartManual(ctx, otherStore, otherVault, "Sender")
	require.NoError(t, err)
	defer senderManager.Stop()

	var output, errorsOut bytes.Buffer
	copied := make(chan string, 1)
	cmd := &SyncCmd{command: &command{out: &output, errOut: &errorsOut, clipboard: func(_ context.Context, data []byte) error {
		copied <- string(data)
		return nil
	}}}
	input := make(chan syncInput, 4)
	input <- syncInput{line: "c"}
	input <- syncInput{line: "v"}
	done := make(chan error, 1)
	go func() { done <- cmd.waitForSyncConnection(ctx, receiver, vault, nil, input, link.String()) }()
	select {
	case value := <-copied:
		require.Equal(t, link.String(), value)
	case <-ctx.Done():
		t.Fatal("clipboard action timed out")
	}
	_, peer, err := sender.ConnectLink(ctx, link)
	require.NoError(t, err)
	peer.IP = net.ParseIP("127.0.0.1")
	pairing, err := sender.RequestPairing(ctx, peer)
	require.NoError(t, err)
	localReference, remoteReference := sender.VaultReferences()
	require.Equal(t, "Sender", localReference.Name)
	require.Equal(t, "Receiver", remoteReference.Name)
	input <- syncInput{line: "n"}
	require.Error(t, pairing.Confirm(ctx, pairing.Code()))
	cancel()
	require.NoError(t, <-done)
	require.Contains(t, output.String(), "Pairing request from Sender")
	require.Contains(t, output.String(), "Receiver (")
	require.Contains(t, output.String(), "Sender:      Sender (")
	require.NotContains(t, output.String(), localReference.ID)
	require.NotContains(t, output.String(), remoteReference.ID)
}

func TestAcceptedIncomingPairingReturnsSyncOutcomeError(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	receiverStore, receiverVault := makeSyncTestVault(t, "Receiver")
	defer receiverStore.Close()
	plannerStore, plannerVault := makeSyncTestVault(t, "Planner")
	defer plannerStore.Close()
	var receiverManager, plannerManager pawsync.Manager
	receiver, err := receiverManager.StartManual(ctx, receiverStore, receiverVault, "Receiver")
	require.NoError(t, err)
	defer receiverManager.Stop()
	planner, err := plannerManager.StartManual(ctx, plannerStore, plannerVault, "Planner")
	require.NoError(t, err)
	defer plannerManager.Stop()
	public, _, err := receiverStore.LoadOrCreateSyncIdentity(receiverVault)
	require.NoError(t, err)
	link := pawsync.ConnectionLink{Host: "127.0.0.1", Port: receiver.Port(), Fingerprint: pawsync.Fingerprint(public)}
	_, peer, err := planner.ConnectLink(ctx, link)
	require.NoError(t, err)
	input := make(chan syncInput)
	var output, errorsOut bytes.Buffer
	cmd := &SyncCmd{command: &command{out: &output, errOut: &errorsOut}}
	done := make(chan error, 1)
	go func() { done <- cmd.waitForSyncConnection(ctx, receiver, receiverVault, nil, input, link.String()) }()
	pairing, err := planner.RequestPairing(ctx, peer)
	require.NoError(t, err)
	input <- syncInput{line: "y"}
	require.NoError(t, pairing.Confirm(ctx, pairing.Code()))
	cancel()
	require.ErrorIs(t, <-done, context.Canceled)
}

func makeSyncTestVault(t *testing.T, name string) (*storage.FileStorage, *model.Vault) {
	t.Helper()
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	vault := test.CreateVault(t, store, name)
	return store, vault
}

func TestSyncPlannerShowsBothVaultsBeforeConfirmation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	localStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer localStore.Close()
	localVault := test.CreateVault(t, localStore, "a")
	var localManager pawsync.Manager
	local, err := localManager.StartManual(ctx, localStore, localVault, "Local")
	require.NoError(t, err)
	defer localManager.Stop()
	remoteStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer remoteStore.Close()
	remoteVault := test.CreateVault(t, remoteStore, "Personal")
	var remoteManager pawsync.Manager
	remote, err := remoteManager.StartManual(ctx, remoteStore, remoteVault, "fedora")
	require.NoError(t, err)
	defer remoteManager.Stop()
	public, _, err := remoteStore.LoadOrCreateSyncIdentity(remoteVault)
	require.NoError(t, err)
	link := pawsync.ConnectionLink{
		Host:        "127.0.0.1",
		Port:        remote.Port(),
		Fingerprint: pawsync.Fingerprint(public),
	}
	_, peer, err := local.ConnectLink(ctx, link)
	require.NoError(t, err)
	peer.Name = "fedora"

	var output, errOutput bytes.Buffer
	cmd := &SyncCmd{command: &command{storage: localStore, out: &output, errOut: &errOutput}}
	result := make(chan error, 1)
	go func() {
		result <- cmd.syncWithPeer(ctx, local, localVault, peer, nil, make(chan syncInput))
	}()
	select {
	case request := <-remote.PairingRequests():
		require.Equal(t, localVault.Accounts[0].ID, request.Vault.ID)
		require.NoError(t, request.Decline())
	case <-ctx.Done():
		t.Fatal("planner did not send a pairing request")
	}
	require.Error(t, <-result)
	localID := storage.FormatVaultID(localVault.Accounts[0].ID)
	remoteID := storage.FormatVaultID(remoteVault.Accounts[0].ID)
	require.Contains(t, output.String(), "Pairing with fedora")
	require.Contains(t, output.String(), fmt.Sprintf("This device: a (%s)", localID[:7]))
	require.Contains(t, output.String(), fmt.Sprintf("fedora:      Personal (%s)", remoteID[:7]))
}

func TestSyncVaultSummaryShowsDistinctReferences(t *testing.T) {
	local := pawsync.VaultReference{Name: "Personal", ID: testVaultID(1, 1)}
	remote := pawsync.VaultReference{Name: "Personal", ID: testVaultID(1, 2)}
	summary := formatSyncVaultSummary("fedora", local, remote)
	localID := storage.FormatVaultID(local.ID)
	remoteID := storage.FormatVaultID(remote.ID)
	require.Contains(t, summary, fmt.Sprintf("This device: Personal (%s)", localID[:7]))
	require.Contains(t, summary, fmt.Sprintf("fedora: Personal (%s)", remoteID[:7]))
	require.NotContains(t, summary, local.ID)
	require.NotContains(t, summary, remote.ID)
}

func TestFormatPairingVaultReferencesShowsBothVaults(t *testing.T) {
	local := pawsync.VaultReference{Name: "a", ID: testVaultID(1, 1)}
	remote := pawsync.VaultReference{Name: "Personal", ID: testVaultID(1, 2)}
	localID := storage.FormatVaultID(local.ID)
	remoteID := storage.FormatVaultID(remote.ID)

	got := formatPairingVaultReferences("fedora", local, remote)

	require.Equal(t, fmt.Sprintf("This device: a (%s)\nfedora:      Personal (%s)", localID[:7], remoteID[:7]), got)
	require.NotContains(t, got, local.ID)
	require.NotContains(t, got, remote.ID)

	clone := formatPairingVaultReferences("phoenix", pawsync.VaultReference{}, remote)
	require.Contains(t, clone, "This device: New vault (created after pairing)")
	require.Contains(t, clone, "phoenix:     Personal (")
}

func TestPastedSyncLinkStartsPairing(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	localStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer localStore.Close()
	localVault := test.CreateVault(t, localStore, "Local")
	var localManager pawsync.Manager
	local, err := localManager.StartManual(ctx, localStore, localVault, "Local")
	require.NoError(t, err)
	defer localManager.Stop()
	remoteStore, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer remoteStore.Close()
	remoteVault := test.CreateVault(t, remoteStore, "Remote")
	var remoteManager pawsync.Manager
	remote, err := remoteManager.StartManual(ctx, remoteStore, remoteVault, "Remote")
	require.NoError(t, err)
	defer remoteManager.Stop()
	public, _, err := remoteStore.LoadOrCreateSyncIdentity(remoteVault)
	require.NoError(t, err)
	link := pawsync.ConnectionLink{
		Host:        "127.0.0.1",
		Port:        remote.Port(),
		Fingerprint: pawsync.Fingerprint(public),
	}
	var output, errorsOut bytes.Buffer
	cmd := &SyncCmd{command: &command{storage: localStore, out: &output, errOut: &errorsOut}}
	input := make(chan syncInput, 1)
	input <- syncInput{line: link.String()}
	done := make(chan error, 1)
	go func() { done <- cmd.waitForSyncConnection(ctx, local, localVault, nil, input, link.String()) }()
	select {
	case request := <-remote.PairingRequests():
		require.Equal(t, "Local", request.Name)
		require.NoError(t, request.Decline())
	case <-ctx.Done():
		t.Fatal("pasted link did not start pairing")
	}
	require.Error(t, <-done)
	require.Contains(t, output.String(), "Connecting...")
}

func TestSyncLinkScreenAndActions(t *testing.T) {
	link := "paw://192.168.0.9:61654/sync?fp=7a2a9d534f4b092dcc8e5a53f644508ed498fdd0203f387f717c74f8b63c118d"
	var out, errOut bytes.Buffer
	var copied string
	cmd := &SyncCmd{command: &command{out: &out, errOut: &errOut, clipboard: func(_ context.Context, data []byte) error {
		copied = string(data)
		return nil
	}}}
	require.NoError(t, cmd.showSyncLink(link))
	require.Contains(t, out.String(), link)
	require.NotContains(t, out.String(), "█")
	require.True(t, strings.HasSuffix(out.String(), "> "))
	for _, input := range []string{"c", "C"} {
		out.Reset()
		copied = ""
		_, connect, err := cmd.handleSyncLinkInput(context.Background(), link, input)
		require.NoError(t, err)
		require.False(t, connect)
		require.Equal(t, link, copied)
		require.Contains(t, out.String(), "Connection link copied.")
	}
	cmd.clipboard = func(context.Context, []byte) error { return errors.New("unavailable") }
	out.Reset()
	_, connect, err := cmd.handleSyncLinkInput(context.Background(), link, "c")
	require.NoError(t, err)
	require.False(t, connect)
	require.Contains(t, errOut.String(), "Could not copy")
	require.NotContains(t, out.String(), "Connection link copied.")
	for _, input := range []string{"v", "V"} {
		out.Reset()
		_, connect, err = cmd.handleSyncLinkInput(context.Background(), link, input)
		require.NoError(t, err)
		require.False(t, connect)
		require.Contains(t, out.String(), "█")
	}
	manual, connect, err := cmd.handleSyncLinkInput(context.Background(), link, link)
	require.NoError(t, err)
	require.True(t, connect)
	require.Equal(t, link, manual.String())
	out.Reset()
	_, connect, err = cmd.handleSyncLinkInput(context.Background(), link, "invalid")
	require.NoError(t, err)
	require.False(t, connect)
	require.Contains(t, out.String(), "Invalid connection link.")
}

func TestRenderSyncQRUsesEncodedLinkAndQuietZone(t *testing.T) {
	link := "paw://192.168.0.9:61654/sync?fp=7a2a9d534f4b092dcc8e5a53f644508ed498fdd0203f387f717c74f8b63c118d"
	code, err := qr.Encode(link, qr.L)
	require.NoError(t, err)
	rendered := renderSyncQR(code)
	require.Contains(t, rendered, "█")
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(rendered, "")
	lines := strings.Split(strings.TrimSuffix(plain, "\n"), "\n")
	require.Equal(t, code.Size+8, len([]rune(lines[0])))
	require.Equal(t, (code.Size+9)/2, len(lines))
}

func TestFormatSyncPeerFoundShowsTrustStatus(t *testing.T) {
	peer := pawsync.Peer{Name: "fedora", Fingerprint: "fingerprint"}
	require.Equal(t, "Found: fedora (New)", formatSyncPeerFound(peer, nil))
	require.Equal(t, "Found: fedora (Trusted)", formatSyncPeerFound(peer, map[string]bool{"fingerprint": true}))
}

func TestAskSyncYesNoRequiresExplicitYes(t *testing.T) {
	for _, answer := range []string{"", "n"} {
		t.Run("answer="+answer, func(t *testing.T) {
			var output bytes.Buffer
			cmd := &SyncCmd{command: &command{out: &output, errOut: &bytes.Buffer{}}}
			input := make(chan syncInput, 1)
			input <- syncInput{line: answer}

			confirmed, err := cmd.askSyncYesNo(context.Background(), nil, input, nil, nil, "Connect to fedora? [y/N]")

			require.NoError(t, err)
			require.False(t, confirmed)
			require.Contains(t, output.String(), "Connect to fedora? [y/N]")
		})
	}
	input := make(chan syncInput, 1)
	input <- syncInput{line: "Y"}
	cmd := &SyncCmd{command: &command{out: &bytes.Buffer{}, errOut: &bytes.Buffer{}}}
	confirmed, err := cmd.askSyncYesNo(context.Background(), nil, input, nil, nil, "Connect to fedora? [y/N]")
	require.NoError(t, err)
	require.True(t, confirmed)
}

func TestCompletedSyncOperationsExcludesSkippedEntries(t *testing.T) {
	entries := []pawsync.PlanEntry{
		{Decision: pawsync.CopyLocalToRemote},
		{Decision: pawsync.Skip},
		{Decision: pawsync.DeleteRemote},
	}
	require.Equal(t, 2, completedSyncOperations(entries))
}

func TestSyncDiffCounts(t *testing.T) {
	diff := []pawsync.Difference{
		{Kind: pawsync.Identical},
		{Kind: pawsync.LocalOnly},
		{Kind: pawsync.RemoteOnly},
		{Kind: pawsync.ContentDifferent},
		{Kind: pawsync.StateDifferent},
	}
	identical, local, remote, different := syncDiffCounts(diff)
	require.Equal(t, 1, identical)
	require.Equal(t, 1, local)
	require.Equal(t, 1, remote)
	require.Equal(t, 2, different)
}

func TestSyncProgressLabelsShowDirectionAndType(t *testing.T) {
	item := "Login / GitHub"
	require.Equal(t, item+" → fedora", syncProgressLabel(pawsync.PlanEntry{Decision: pawsync.CopyLocalToRemote}, item, "fedora"))
	require.Equal(t, item+" → this device", syncProgressLabel(pawsync.PlanEntry{Decision: pawsync.CopyRemoteToLocal}, item, "fedora"))
	require.Equal(t, "Delete "+item+" from fedora", syncProgressLabel(pawsync.PlanEntry{Decision: pawsync.DeleteRemote}, item, "fedora"))
	require.Equal(t, "Delete "+item+" from this device", syncProgressLabel(pawsync.PlanEntry{Decision: pawsync.DeleteLocal}, item, "fedora"))
}

func TestSyncPlanSummaryOmitsZeroCategories(t *testing.T) {
	summary := pawsync.PlanSummary{CopyLocalToRemote: 4, CopyRemoteToLocal: 1}
	require.Equal(t, "  Copy to fedora: 4\n  Copy to this device: 1\n", formatSyncPlanSummary("fedora", summary))
}

func TestReviewSyncPlanFinishesWithoutExecutorWhenThereAreNoChanges(t *testing.T) {
	local, remote := &pawsync.Manifest{}, &pawsync.Manifest{}
	plan, err := pawsync.BuildPlan(nil, local, remote)
	require.NoError(t, err)
	var output bytes.Buffer
	cmd := &SyncCmd{command: &command{out: &output, errOut: &bytes.Buffer{}}}

	completed, err := cmd.reviewSyncPlan(context.Background(), nil, pawsync.Peer{Name: "fedora"}, local, remote, nil, plan, nil)

	require.NoError(t, err)
	require.True(t, completed)
	require.Equal(t, "No changes to apply.\n", output.String())
}

func TestSyncPlannerReportsNoopAndCancellationToResponder(t *testing.T) {
	for _, tc := range []struct {
		name      string
		choice    string
		cancelled bool
	}{
		{name: "identical"},
		{name: "all skipped", choice: "s"},
		{name: "review cancelled", choice: "q", cancelled: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			plannerStore, err := storage.NewFileStorage(t.TempDir())
			require.NoError(t, err)
			defer plannerStore.Close()
			plannerVault := test.CreateVault(t, plannerStore, "Planner")
			responderStore, err := storage.NewFileStorage(t.TempDir())
			require.NoError(t, err)
			defer responderStore.Close()
			responderVault := test.CreateVault(t, responderStore, "Responder")
			if tc.choice != "" {
				item, err := model.NewNote("Remote note", "contents")
				require.NoError(t, err)
				require.NoError(t, responderStore.StoreItem(responderVault, item))
			}
			plannerPublic, _, err := plannerStore.LoadOrCreateSyncIdentity(plannerVault)
			require.NoError(t, err)
			responderPublic, _, err := responderStore.LoadOrCreateSyncIdentity(responderVault)
			require.NoError(t, err)
			require.NoError(t, plannerStore.AddSyncPeer(plannerVault, storage.SyncPeer{
				PublicKey: responderPublic.Bytes(),
				Name:      "Responder",
			}))
			require.NoError(t, responderStore.AddSyncPeer(responderVault, storage.SyncPeer{
				PublicKey: plannerPublic.Bytes(),
				Name:      "Planner",
			}))
			var plannerManager, responderManager pawsync.Manager
			planner, err := plannerManager.StartManual(ctx, plannerStore, plannerVault, "Planner")
			require.NoError(t, err)
			defer plannerManager.Stop()
			responder, err := responderManager.StartManual(ctx, responderStore, responderVault, "Responder")
			require.NoError(t, err)
			defer responderManager.Stop()
			link := pawsync.ConnectionLink{
				Host:        "127.0.0.1",
				Port:        responder.Port(),
				Fingerprint: pawsync.Fingerprint(responderPublic),
			}
			_, peer, err := planner.ConnectLink(ctx, link)
			require.NoError(t, err)
			peer.Name = "Responder"
			input := make(chan syncInput, 1)
			if tc.choice != "" {
				input <- syncInput{line: tc.choice}
			}
			var output, errorsOut bytes.Buffer
			cmd := &SyncCmd{command: &command{storage: plannerStore, out: &output, errOut: &errorsOut}}
			err = cmd.syncWithPeer(ctx, planner, plannerVault, peer, map[string]bool{peer.Fingerprint: true}, input)
			if tc.cancelled {
				require.ErrorIs(t, err, errCanceled)
			} else {
				require.NoError(t, err)
			}
			select {
			case successful := <-responder.SyncOutcomes():
				require.Equal(t, !tc.cancelled, successful)
				require.Equal(t, tc.cancelled, responder.SyncOutcomeCancelled())
			case <-ctx.Done():
				t.Fatal("responder did not receive a terminal outcome")
			}
			manifest, err := pawsync.BuildManifest(ctx, plannerStore, plannerVault)
			require.NoError(t, err)
			require.Empty(t, manifest.Items)
		})
	}
}
