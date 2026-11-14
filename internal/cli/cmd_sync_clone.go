// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	pawsync "go.lucor.dev/paw/internal/sync"
)

// runClone receives a vault from a peer and creates a local copy.
func (cmd *SyncCmd) runClone(ctx context.Context) (result error) {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		host = "Paw device"
	}
	var manager pawsync.Manager
	session, err := manager.StartClone(ctx, cmd.storage, host, cmd.link == "")
	if err != nil {
		return fmt.Errorf("start clone: %w", err)
	}
	defer func() { result = errors.Join(result, manager.Stop(), session.Wait()) }()
	var peer pawsync.Peer
	if cmd.link != "" {
		link, err := pawsync.ParseConnectionLink(cmd.link)
		if err != nil {
			return err
		}
		_, peer, err = session.ConnectLink(ctx, link)
		if err != nil {
			return err
		}
	} else {
		fmt.Fprintln(cmd.out, "Looking for devices. Ready to be found.")
		select {
		case <-ctx.Done():
			return ctx.Err()
		case found, ok := <-session.Peers():
			if !ok {
				return fmt.Errorf("sync discovery stopped")
			}
			peer = found
		}
		fmt.Fprintf(cmd.out, "Found: %s\n", peer.Name)
		confirmed, err := cmd.prompter.askYesNo("Connect to "+peer.Name+"?", false)
		if err != nil || !confirmed {
			return err
		}
	}
	fmt.Fprintf(cmd.out, "Pairing with %s\n", peer.Name)
	pairing, err := session.RequestPairing(ctx, peer)
	if err != nil {
		return err
	}
	localVault, remoteVault := session.VaultReferences()
	fmt.Fprintf(cmd.out, "%s\n\nVerification code: %s\nConfirm this code on %s.\nWaiting for confirmation...\n", formatPairingVaultReferences(peer.Name, localVault, remoteVault), formatSyncCode(pairing.Code()), peer.Name)
	if err := pairing.Confirm(ctx, pairing.Code()); err != nil {
		return fmt.Errorf("pairing failed: %w", err)
	}
	successful := false
	defer func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := session.NotifySyncComplete(notifyCtx, peer, successful); err != nil {
			fmt.Fprintf(cmd.errOut, "Could not notify %s: %v\n", peer.Name, err)
		}
	}()
	info, err := session.RemoteVaultInfo(ctx)
	if err != nil {
		return err
	}
	if info.Name == "" || info.ID == "" {
		return fmt.Errorf("source vault information is incomplete")
	}
	fmt.Fprintf(cmd.out, "\nConnected to %s.\n\nVault: %s (%s)\n", peer.Name, info.Name, shortVaultID(info.ID))
	cloneName, err := promptCloneVaultName(cmd.prompter, info.Name)
	if err != nil {
		return err
	}
	confirmed, err := cmd.prompter.askYesNo("Clone this vault to this device?", false)
	if err != nil || !confirmed {
		return err
	}
	password, err := cmd.prompter.askPassword("New vault password")
	if err != nil {
		return err
	}
	confirm, err := cmd.prompter.askPassword("Confirm password")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("passwords do not match")
	}
	fmt.Fprintf(cmd.out, "Creating %s...\n", cloneName)
	vault, err := cmd.storage.CreateCloneVault(cloneName, password)
	if err != nil {
		return err
	}
	defer func() {
		if !successful {
			if err := cmd.storage.DeleteVault(vault.Accounts[0].ID); err != nil {
				result = errors.Join(result, fmt.Errorf("delete incomplete clone: %w", err))
			}
		}
	}()
	if err := session.AttachCloneVault(vault); err != nil {
		return fmt.Errorf("attach clone vault: %w", err)
	}
	local, err := pawsync.BuildManifest(ctx, cmd.storage, vault)
	if err != nil {
		return err
	}
	remote, err := session.RemoteManifest(ctx, peer)
	if err != nil {
		return fmt.Errorf("read source manifest: %w", err)
	}
	plan, err := pawsync.BuildClonePlan(local, remote)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.out, "Receiving items...")
	index := 0
	execution, err := session.ExecuteWithProgress(ctx, peer, plan, func(entry pawsync.PlanEntry, started bool, operationErr error) {
		if started {
			index++
			fmt.Fprintf(cmd.out, "[%d/%d] %s... ", index, len(plan.Entries), syncItemName(manifestItem(local, remote, entry.ItemID)))
		} else if operationErr != nil {
			fmt.Fprintf(cmd.out, "Failed: %v\n", operationErr)
		} else {
			fmt.Fprintln(cmd.out, "Done")
		}
	})
	if err != nil {
		return fmt.Errorf("clone interrupted after %d of %d items: %w", len(execution.Completed), len(plan.Entries), err)
	}
	if err := cmd.storage.CompleteClone(vault.Accounts[0].ID); err != nil {
		return fmt.Errorf("complete cloned vault: %w", err)
	}
	successful = true
	fmt.Fprintf(cmd.out, "Vault %s is ready.\n%d items transferred.\n", cloneName, len(execution.Completed))
	return nil
}

// promptCloneVaultName asks for a valid local name for the cloned vault.
func promptCloneVaultName(prompter *prompter, sourceName string) (string, error) {
	name, err := prompter.askWithDefault("New vault name", sourceName)
	if err != nil {
		return "", err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("vault name is required")
	}
	return name, nil
}
