// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	pawsync "go.lucor.dev/paw/internal/sync"
	"rsc.io/qr"
)

type syncInput struct {
	line string
	err  error
}

var errSyncResponderSelected = errors.New("incoming Sync session selected")

// renderSyncQR renders a QR code as terminal block characters.
func renderSyncQR(code *qr.Code) string {
	const quiet = 4
	var output strings.Builder
	blocks := []rune(" ▀▄█")
	for y := -quiet; y < code.Size+quiet; y += 2 {
		for x := -quiet; x < code.Size+quiet; x++ {
			index := 0
			if code.Black(x, y) {
				index |= 1
			}
			if code.Black(x, y+1) {
				index |= 2
			}
			output.WriteRune(blocks[index])
		}
		output.WriteByte('\n')
	}
	return output.String()
}

// newSyncCommand constructs the vault-synchronization command.
func newSyncCommand(base *command) *urfave.Command {
	cmd := &SyncCmd{command: base}
	c := &urfave.Command{
		Name:        "sync",
		Usage:       "Synchronize an unlocked vault with another Paw device",
		ArgsUsage:   "VAULT",
		Description: vaultReferenceHelp,
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "link", Usage: "Connect directly using a Paw Sync link"},
			&urfave.BoolFlag{Name: "clone", Usage: "Clone a vault from another device"},
			&urfave.BoolFlag{Name: "reconcile", Usage: "Match and consolidate items with different IDs"},
		},
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// SyncCmd coordinates one interactive Sync session for an existing unlocked vault.
type SyncCmd struct {
	*command
	vaultName string
	link      string
	reconcile bool
	clone     bool
}

// Validate checks the sync mode, arguments, and incompatible flags.
func (cmd *SyncCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 1); err != nil {
		return ctx, err
	}
	clone, reconcile := c.Bool("clone"), c.Bool("reconcile")
	if clone && c.Args().Len() != 0 {
		return ctx, fmt.Errorf("clone does not accept a vault argument")
	}
	if clone && reconcile {
		return ctx, fmt.Errorf("reconcile cannot be combined with clone")
	}
	if !clone && c.Args().Len() != 1 {
		return ctx, fmt.Errorf("invalid arguments: a vault argument is required")
	}
	cmd.vaultName = c.Args().First()
	cmd.link = c.String("link")
	cmd.reconcile = reconcile
	cmd.clone = clone
	return ctx, nil
}

// Run starts vault synchronization and handles incoming or outgoing connections.
func (cmd *SyncCmd) Run(ctx context.Context, _ *urfave.Command) (runErr error) {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	if cmd.clone {
		return cmd.runClone(ctx)
	}
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.getenv(paw.ENV_SESSION))
	if err != nil {
		return fmt.Errorf("unlock vault: %w", err)
	}
	vault, err := cmd.loadVault(key)
	if err != nil {
		return err
	}
	state, err := cmd.storage.LoadAppState()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	host := state.Settings.DeviceName
	manager := &pawsync.Manager{}
	var session *pawsync.Session
	if cmd.link != "" {
		session, err = manager.StartManual(ctx, cmd.storage, vault, host)
	} else {
		session, err = manager.Start(ctx, cmd.storage, vault, host)
	}
	if err != nil {
		return fmt.Errorf("start Sync: %w", err)
	}
	defer func() {
		if errors.Is(runErr, context.Canceled) {
			runErr = nil
		}
		runErr = errors.Join(runErr, manager.Stop())
		runErr = errors.Join(runErr, session.Wait())
	}()

	items := make(chan syncInput, 1)
	inputCtx, stopInput := context.WithCancel(ctx)
	defer stopInput()
	go func() {
		reader := bufio.NewReader(cmd.prompter.input)
		for {
			line, err := reader.ReadString('\n')
			line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
			select {
			case items <- syncInput{line: line, err: err}:
			case <-inputCtx.Done():
				return
			}
			if err != nil {
				return
			}
		}
	}()

	name, err := vault.Name()
	if err != nil {
		return err
	}
	trusted, err := session.TrustedFingerprints()
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "Sync — %s\n\nLooking for devices. Ready to be found.\n", name)
	if cmd.link != "" {
		manual, err := pawsync.ParseConnectionLink(cmd.link)
		if err != nil {
			return err
		}
		_, peer, err := session.ConnectLink(ctx, manual)
		if err != nil {
			return err
		}
		return cmd.syncWithPeer(ctx, session, vault, peer, trusted, items)
	}
	link, err := session.ConnectionLink()
	if err != nil {
		return err
	}
	return cmd.waitForSyncConnection(ctx, session, vault, trusted, items, link)
}

// waitForSyncConnection handles discovered peers, pairing requests, and pasted links.
func (cmd *SyncCmd) waitForSyncConnection(ctx context.Context, session *pawsync.Session, vault *model.Vault, trusted map[string]bool, items <-chan syncInput, link string) error {
	cmd.showSyncLink(link)
	pairingRequests := session.PairingRequests()
	incomingSyncs := session.IncomingSyncs()
	for {
		select {
		case <-ctx.Done():
			return nil
		case request, ok := <-pairingRequests:
			if !ok {
				pairingRequests = nil
				continue
			}
			selected, err := cmd.handleIncomingPairing(ctx, session, request, items)
			if selected {
				return err
			}
			if err != nil {
				fmt.Fprintf(cmd.errOut, "Pairing request failed: %v\n", err)
			}
			fmt.Fprint(cmd.out, "> ")
		case device, ok := <-incomingSyncs:
			if !ok {
				incomingSyncs = nil
				continue
			}
			fmt.Fprintf(cmd.out, "\nSyncing with %s...\n", device)
			return cmd.waitForSyncOutcome(ctx, session)
		case peer, ok := <-session.Peers():
			if !ok {
				return fmt.Errorf("sync discovery stopped")
			}
			slog.Debug("CLI received discovered peer", "component", "sync.cli", "device", peer.Name, "instance", peer.ID, "port", peer.Port)
			if peer.Name == "" {
				peer.Name = "Unknown Paw device"
			}
			fmt.Fprintf(cmd.out, "\n%s\n", formatSyncPeerFound(peer, trusted))
			confirmed, err := cmd.askSyncYesNo(ctx, session, items, pairingRequests, incomingSyncs, "Connect to "+peer.Name+"? [y/N]")
			if errors.Is(err, errSyncResponderSelected) {
				return nil
			}
			if err != nil {
				return err
			}
			if !confirmed {
				return nil
			}
			return cmd.syncWithPeer(ctx, session, vault, peer, trusted, items)
		case input := <-items:
			if input.err != nil {
				return input.err
			}
			manual, connect := cmd.handleSyncLinkInput(ctx, link, input.line)
			if connect {
				fmt.Fprintln(cmd.out, "Connecting...")
				_, peer, err := session.ConnectLink(ctx, manual)
				if err != nil {
					fmt.Fprintf(cmd.errOut, "Connection failed: %v\n", err)
				} else {
					return cmd.syncWithPeer(ctx, session, vault, peer, trusted, items)
				}
			}
			fmt.Fprint(cmd.out, "> ")
		}
	}
}

// showSyncLink displays a synchronization link and its terminal QR code.
func (cmd *SyncCmd) showSyncLink(link string) {
	fmt.Fprintf(cmd.out, "\nConnection link:\n%s\n\n  [c] Copy link\n  [v] View QR code\n\nPaste a connection link, or wait:\n> ", link)
}

// handleSyncLinkInput parses a pasted link and reports whether it started pairing.
func (cmd *SyncCmd) handleSyncLinkInput(ctx context.Context, link, input string) (pawsync.ConnectionLink, bool) {
	value := strings.TrimSpace(input)
	switch strings.ToLower(value) {
	case "c":
		copyCtx, cancel := context.WithTimeout(ctx, clipboardWriteTimeout)
		defer cancel()
		if err := cmd.clipboard(copyCtx, []byte(link)); err != nil {
			fmt.Fprintf(cmd.errOut, "Could not copy connection link: %v\n", err)
		} else {
			fmt.Fprintln(cmd.out, "Connection link copied.")
		}
	case "v":
		code, err := qr.Encode(link, qr.L)
		if err != nil {
			fmt.Fprintf(cmd.errOut, "Could not display QR code: %v\n", err)
		} else {
			fmt.Fprintln(cmd.out, renderSyncQR(code))
		}
	default:
		manual, err := pawsync.ParseConnectionLink(value)
		if err == nil {
			return manual, true
		}
		fmt.Fprintln(cmd.out, "Invalid connection link.")
	}
	return pawsync.ConnectionLink{}, false
}

// formatSyncPeerFound describes a discovered peer and its trust status.
func formatSyncPeerFound(peer pawsync.Peer, trusted map[string]bool) string {
	status := "New"
	if trusted[peer.Fingerprint] {
		status = "Trusted"
	}
	return fmt.Sprintf("Found: %s (%s)", peer.Name, status)
}

// handleIncomingPairing asks whether to accept a peer's pairing request.
func (cmd *SyncCmd) handleIncomingPairing(ctx context.Context, session *pawsync.Session, request pawsync.PairingRequest, input <-chan syncInput) (bool, error) {
	code, err := request.VerificationCode()
	if err != nil {
		_ = request.Decline()
		return false, err
	}
	localVault, _ := session.VaultReferences()
	fmt.Fprintf(cmd.out, "\nPairing request from %s\n\n%s\n\nVerification code: %s\n", request.Name, formatPairingVaultReferences(request.Name, localVault, request.Vault), formatSyncCode(code))
	accepted, err := cmd.askSyncYesNo(ctx, session, input, nil, nil, "Does this code match the one on "+request.Name+"? Confirm pairing? [y/N]")
	if err != nil {
		_ = request.Decline()
		return false, err
	}
	if !accepted {
		return false, request.Decline()
	}
	if err := request.Accept(); err != nil {
		return false, err
	}
	fmt.Fprintf(cmd.out, "Waiting for %s to continue...\n", request.Name)
	if err := request.Wait(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Fprintf(cmd.errOut, "Pairing with %s failed: %v\n", request.Name, err)
		}
		return true, nil
	}
	fmt.Fprintf(cmd.out, "Paired with %s.\n", request.Name)
	return true, cmd.waitForSyncOutcome(ctx, session)
}

// waitForSyncOutcome waits for the active synchronization session to finish.
func (cmd *SyncCmd) waitForSyncOutcome(ctx context.Context, session *pawsync.Session) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-session.SyncConnectionLost():
		return fmt.Errorf("connection lost; sync outcome is unknown")
	case successful := <-session.SyncOutcomes():
		if successful {
			fmt.Fprintln(cmd.out, "Sync completed.")
			return nil
		}
		if session.SyncOutcomeCancelled() {
			fmt.Fprintln(cmd.out, "Sync cancelled by the other device.")
			return nil
		}
		fmt.Fprintln(cmd.out, "Sync did not complete.")
		return nil
	}
}

// askSyncYesNo prompts for confirmation while continuing to handle incoming sync requests.
func (cmd *SyncCmd) askSyncYesNo(ctx context.Context, session *pawsync.Session, input <-chan syncInput, incoming <-chan pawsync.PairingRequest, incomingSync <-chan string, prompt string) (bool, error) {
	for {
		fmt.Fprintf(cmd.out, "%s ", prompt)
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case request, ok := <-incoming:
			if !ok {
				incoming = nil
				continue
			}
			selected, err := cmd.handleIncomingPairing(ctx, session, request, input)
			if selected {
				return false, errSyncResponderSelected
			}
			if err != nil {
				fmt.Fprintf(cmd.errOut, "Pairing request failed: %v\n", err)
			}
		case device, ok := <-incomingSync:
			if !ok {
				incomingSync = nil
				continue
			}
			fmt.Fprintf(cmd.out, "\nSyncing with %s...\n", device)
			if err := cmd.waitForSyncOutcome(ctx, session); err != nil {
				return false, err
			}
			return false, errSyncResponderSelected
		case answer := <-input:
			if answer.err != nil {
				return false, answer.err
			}
			switch strings.ToLower(strings.TrimSpace(answer.line)) {
			case "y":
				return true, nil
			case "", "n":
				return false, nil
			default:
				fmt.Fprintln(cmd.out, "Please answer y or n.")
			}
		}
	}
}

// completedSyncOperations counts plan entries that finished without being skipped.
func completedSyncOperations(entries []pawsync.PlanEntry) int {
	completed := 0
	for _, entry := range entries {
		if entry.Decision != pawsync.Skip {
			completed++
		}
	}
	return completed
}

// formatSyncCode groups a pairing code into readable chunks.
func formatSyncCode(code string) string {
	if len(code) == 6 {
		return code[:3] + " " + code[3:]
	}
	return code
}

// formatVaultReferences formats both vault references for display.
func formatVaultReferences(local, remote pawsync.VaultReference) (string, string) {
	return formatVaultReference(local, shortVaultID(local.ID)), formatVaultReference(remote, shortVaultID(remote.ID))
}

// formatVaultReference formats a vault name and abbreviated ID.
func formatVaultReference(reference pawsync.VaultReference, prefix string) string {
	if prefix == "" {
		return reference.Name
	}
	return fmt.Sprintf("%s (%s)", reference.Name, prefix)
}

// formatSyncVaultSummary describes the local and remote vaults in a sync session.
func formatSyncVaultSummary(peerName string, local, remote pawsync.VaultReference) string {
	localReference, remoteReference := formatVaultReferences(local, remote)
	return fmt.Sprintf("\nSync with %s\n\nThis device: %s\n%s: %s\n", peerName, localReference, peerName, remoteReference)
}

// formatPairingVaultReferences describes the vaults involved in a pairing request.
func formatPairingVaultReferences(peerName string, local, remote pawsync.VaultReference) string {
	localReference, remoteReference := formatVaultReferences(local, remote)
	if local.Name == "" {
		localReference = "New vault (created after pairing)"
	}
	if remote.Name == "" {
		remoteReference = "New clone (vault not created yet)"
	}
	return fmt.Sprintf("This device: %s\n%-12s %s", localReference, peerName+":", remoteReference)
}

// syncWithPeer establishes a peer session and synchronizes the selected vault.
func (cmd *SyncCmd) syncWithPeer(ctx context.Context, session *pawsync.Session, vault *model.Vault, peer pawsync.Peer, trusted map[string]bool, input <-chan syncInput) (resultErr error) {
	shouldNotify := false
	successful := false
	defer func() {
		if !shouldNotify {
			return
		}
		notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var err error
		if !successful && (resultErr == nil || errors.Is(resultErr, errCanceled) || errors.Is(resultErr, context.Canceled)) {
			err = session.NotifySyncCancelled(notifyCtx, peer)
		} else {
			err = session.NotifySyncComplete(notifyCtx, peer, successful)
		}
		if err != nil {
			fmt.Fprintf(cmd.errOut, "Could not notify %s that Sync ended: %v\n", peer.Name, err)
		}
	}()
	if !trusted[peer.Fingerprint] {
		fmt.Fprintf(cmd.out, "Pairing with %s\n", peer.Name)
		pairing, err := session.RequestPairing(ctx, peer)
		if err != nil {
			return fmt.Errorf("request pairing: %w", err)
		}
		localVault, remoteVault := session.VaultReferences()
		fmt.Fprintf(cmd.out, "\n%s\n\nVerification code: %s\nConfirm this code on %s.\nWaiting for confirmation...\n", formatPairingVaultReferences(peer.Name, localVault, remoteVault), formatSyncCode(pairing.Code()), peer.Name)
		if err := pairing.Confirm(ctx, pairing.Code()); err != nil {
			_ = pairing.Cancel(ctx)
			return fmt.Errorf("confirm pairing: %w", err)
		}
		shouldNotify = true
		fmt.Fprintf(cmd.out, "Paired with %s.\n", peer.Name)
	}
	local, err := pawsync.BuildManifest(ctx, cmd.storage, vault)
	if err != nil {
		return err
	}
	if trusted[peer.Fingerprint] {
		shouldNotify = true
	}
	remote, err := session.RemoteManifest(ctx, peer)
	if err != nil {
		return err
	}
	if _, err := session.RemoteVaultInfo(ctx); err != nil {
		return err
	}
	if cmd.reconcile {
		localVault, remoteVault := session.VaultReferences()
		fmt.Fprint(cmd.out, formatSyncVaultSummary(peer.Name, localVault, remoteVault))
		successful, err = cmd.reviewReconciliation(ctx, session, peer, vault, local, remote, input)
		return err
	}
	diff, err := pawsync.BuildDiff(local, remote)
	if err != nil {
		return err
	}
	plan, err := pawsync.BuildPlan(diff, local, remote)
	if err != nil {
		return err
	}
	if err := plan.RequireDecisions(); err != nil {
		return err
	}
	localVault, remoteVault := session.VaultReferences()
	fmt.Fprint(cmd.out, formatSyncVaultSummary(peer.Name, localVault, remoteVault))
	successful, err = cmd.reviewSyncPlan(ctx, session, peer, local, remote, diff, plan, input)
	return err
}

// reviewSyncPlan presents planned changes and asks the user whether to continue.
func (cmd *SyncCmd) reviewSyncPlan(ctx context.Context, session *pawsync.Session, peer pawsync.Peer, local, remote *pawsync.Manifest, diff []pawsync.Difference, plan *pawsync.Plan, input <-chan syncInput) (bool, error) {
	if len(plan.Entries) == 0 {
		fmt.Fprintln(cmd.out, "No changes to apply.")
		return true, nil
	}
	identical, localOnly, remoteOnly, different := syncDiffCounts(diff)
	fmt.Fprintf(cmd.out, "\n  %d identical items\n  %d only on this device\n  %d only on %s\n  %d with different versions\n\n%d items require a decision.\n", identical, localOnly, remoteOnly, peer.Name, different, len(plan.Entries))
	for index := range plan.Entries {
		entry := &plan.Entries[index]
		localItem := findManifestItem(local, entry.ItemID)
		remoteItem := findManifestItem(remote, entry.ItemID)
		item := localItem
		if item == nil {
			item = remoteItem
		}
		fmt.Fprintf(cmd.out, "\n[%d/%d] %s\n", index+1, len(plan.Entries), syncItemName(item))
		choice := ""
		switch entry.Kind {
		case pawsync.LocalOnly:
			fmt.Fprintln(cmd.out, "Only on this device")
		case pawsync.RemoteOnly:
			fmt.Fprintf(cmd.out, "Only on %s\n", peer.Name)
		case pawsync.ContentDifferent, pawsync.StateDifferent:
			fmt.Fprintf(cmd.out, "Different versions\n\n%s\n", formatSyncVersionTimes(localItem, remoteItem, peer.Name))
		}
		for _, option := range syncDecisionMenu(entry.Kind, peer.Name) {
			fmt.Fprintf(cmd.out, "\n  %s", option)
		}
		fmt.Fprintln(cmd.out)
		for choice == "" {
			fmt.Fprint(cmd.out, "Choice: ")
			select {
			case <-ctx.Done():
				return false, ctx.Err()
			case answer := <-input:
				if answer.err != nil {
					return false, answer.err
				}
				decision, quit, err := parseSyncDecision(entry.Kind, answer.line)
				if err != nil {
					fmt.Fprintln(cmd.out, err)
					continue
				}
				if quit {
					return false, errCanceled
				}
				if err := plan.SetDecision(entry.ItemID, decision); err != nil {
					return false, err
				}
				choice = answer.line
			}
		}
	}
	if err := plan.Validate(); err != nil {
		return false, err
	}
	summary := plan.Summary()
	return cmd.executeReviewedSyncPlan(ctx, session, peer, local, remote, plan, summary, input)
}

// syncDiffCounts groups differences by whether items match or exist on either side.
func syncDiffCounts(diff []pawsync.Difference) (identical, localOnly, remoteOnly, different int) {
	for _, item := range diff {
		switch item.Kind {
		case pawsync.Identical:
			identical++
		case pawsync.LocalOnly:
			localOnly++
		case pawsync.RemoteOnly:
			remoteOnly++
		default:
			different++
		}
	}
	return identical, localOnly, remoteOnly, different
}

// executeReviewedSyncPlan runs a plan after user approval and reports its progress.
func (cmd *SyncCmd) executeReviewedSyncPlan(
	ctx context.Context,
	session *pawsync.Session,
	peer pawsync.Peer,
	local, remote *pawsync.Manifest,
	plan *pawsync.Plan,
	summary pawsync.PlanSummary,
	input <-chan syncInput,
) (bool, error) {
	operationCount := summary.CopyLocalToRemote + summary.CopyRemoteToLocal + summary.DeleteLocal + summary.DeleteRemote
	if operationCount == 0 {
		fmt.Fprintln(cmd.out, "No changes to apply.")
		return true, nil
	}
	fmt.Fprintf(cmd.out, "\nReady to sync with %s\n\n%s", peer.Name, formatSyncPlanSummary(peer.Name, summary))
	confirmed, err := cmd.askSyncYesNo(ctx, nil, input, nil, nil, "Proceed? [y/N]")
	if err != nil {
		return false, err
	}
	if !confirmed {
		return false, nil
	}
	fmt.Fprintf(cmd.out, "\nSyncing with %s...\n", peer.Name)
	operation := 0
	result, err := session.ExecuteWithProgress(ctx, peer, plan, func(entry pawsync.PlanEntry, started bool, operationErr error) {
		if entry.Decision == pawsync.Skip {
			return
		}
		if started {
			operation++
			fmt.Fprintf(cmd.out, "[%d/%d] %s... ", operation, operationCount, syncProgressLabel(entry, syncItemName(manifestItem(local, remote, entry.ItemID)), peer.Name))
			return
		}
		if operationErr == nil {
			fmt.Fprintln(cmd.out, "Done")
			return
		}
		fmt.Fprintf(cmd.out, "Failed: %v\n", operationErr)
	})
	if err != nil {
		fmt.Fprintf(cmd.errOut, "Sync stopped: %v\n\n%d operations completed.\n", err, completedSyncOperations(result.Completed))
		if result.Failed != nil && (result.Failed.Decision == pawsync.CopyLocalToRemote || result.Failed.Decision == pawsync.DeleteRemote) {
			fmt.Fprintln(cmd.errOut, "The current operation may have completed on the other device.")
		}
		fmt.Fprintln(cmd.errOut, "Run Sync again to compare the current vault states.")
		return false, err
	}
	fmt.Fprintf(cmd.out, "\nSync completed.\n%d items synchronized.\n", operationCount)
	return true, nil
}

// syncProgressLabel describes the item and direction of a plan operation.
func syncProgressLabel(entry pawsync.PlanEntry, item, device string) string {
	switch entry.Decision {
	case pawsync.CopyLocalToRemote:
		return item + " → " + device
	case pawsync.CopyRemoteToLocal:
		return item + " → this device"
	case pawsync.DeleteLocal:
		return "Delete " + item + " from this device"
	case pawsync.DeleteRemote:
		return "Delete " + item + " from " + device
	default:
		return item
	}
}

// formatSyncPlanSummary describes the planned changes for a peer.
func formatSyncPlanSummary(device string, summary pawsync.PlanSummary) string {
	var output strings.Builder
	if summary.CopyLocalToRemote > 0 {
		fmt.Fprintf(&output, "  Copy to %s: %d\n", device, summary.CopyLocalToRemote)
	}
	if summary.CopyRemoteToLocal > 0 {
		fmt.Fprintf(&output, "  Copy to this device: %d\n", summary.CopyRemoteToLocal)
	}
	if summary.DeleteLocal > 0 {
		fmt.Fprintf(&output, "  Delete from this device: %d\n", summary.DeleteLocal)
	}
	if summary.DeleteRemote > 0 {
		fmt.Fprintf(&output, "  Delete from %s: %d\n", device, summary.DeleteRemote)
	}
	if summary.Skipped > 0 {
		fmt.Fprintf(&output, "  Skip: %d\n", summary.Skipped)
	}
	return output.String()
}

// manifestItem returns the item for an ID from either manifest.
func manifestItem(local, remote *pawsync.Manifest, id string) *pawsync.ManifestItem {
	item := findManifestItem(local, id)
	if item == nil {
		item = findManifestItem(remote, id)
	}
	return item
}

// findManifestItem looks up an item by ID in a manifest.
func findManifestItem(manifest *pawsync.Manifest, id string) *pawsync.ManifestItem {
	if manifest == nil {
		return nil
	}
	for index := range manifest.Items {
		if manifest.Items[index].ID == id {
			return &manifest.Items[index]
		}
	}
	return nil
}

// syncItemName returns a safe display name for a manifest item.
func syncItemName(item *pawsync.ManifestItem) string {
	if item == nil {
		return "Item"
	}
	title := strings.TrimSpace(item.Title)
	if title == "" {
		title = "Untitled item"
	}
	typeName := item.Type.Label()
	if item.Type.IsZero() {
		typeName = "Item"
	}
	return typeName + " / " + title
}

// formatSyncTimestamp formats an item's modification time for display.
func formatSyncTimestamp(item *pawsync.ManifestItem) string {
	if item == nil || item.ModifiedAt == nil || *item.ModifiedAt == 0 {
		return "Unknown"
	}
	return time.Unix(int64(*item.ModifiedAt), 0).Local().Format("Jan 2, 15:04")
}

// formatSyncVersionTimes displays the local and remote modification times.
func formatSyncVersionTimes(local, remote *pawsync.ManifestItem, remoteName string) string {
	return fmt.Sprintf("This device: %s\n%-12s %s", formatSyncTimestamp(local), remoteName+":", formatSyncTimestamp(remote))
}
