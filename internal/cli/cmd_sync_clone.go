// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	pawsync "go.lucor.dev/paw/internal/sync"
)

// runClone receives a vault from a peer and creates a local copy.
func (cmd *SyncCmd) runClone(ctx context.Context) (result error) {
	state, err := cmd.storage.LoadAppState()
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	host := state.Settings.DeviceName
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
		if err := cmd.Infof("Looking for devices. Ready to be found.\n"); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case found, ok := <-session.Peers():
			if !ok {
				return fmt.Errorf("sync discovery stopped")
			}
			peer = found
		}
		if err := cmd.Infof("Found: %s\n", safeTerminalText(peer.Name)); err != nil {
			return err
		}
		confirmed, err := cmd.prompter.askYesNo("Connect to "+safeTerminalText(peer.Name)+"?", false)
		if err != nil || !confirmed {
			return err
		}
	}
	if err := cmd.Infof("Pairing with %s\n", safeTerminalText(peer.Name)); err != nil {
		return err
	}
	pairing, err := session.RequestPairing(ctx, peer)
	if err != nil {
		return err
	}
	localVault, remoteVault := session.VaultReferences()
	if err := cmd.Infof("%s\n\nVerification code: %s\nConfirm this code on %s.\nWaiting for confirmation...\n", formatPairingVaultReferences(peer.Name, localVault, remoteVault), formatSyncCode(pairing.Code()), safeTerminalText(peer.Name)); err != nil {
		return err
	}
	if err := pairing.Confirm(ctx, pairing.Code()); err != nil {
		return fmt.Errorf("pairing failed: %w", err)
	}
	successful := false
	defer func() {
		notifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := session.NotifySyncComplete(notifyCtx, peer, successful); err != nil {
			if writeErr := cmd.Warnf("Could not notify %s: %s\n", safeTerminalText(peer.Name), safeTerminalText(err.Error())); writeErr != nil {
				result = errors.Join(result, writeErr)
			}
		}
	}()
	info, err := session.RemoteVaultInfo(ctx)
	if err != nil {
		return err
	}
	if info.Name == "" || info.ID == "" {
		return fmt.Errorf("source vault information is incomplete")
	}
	if err := cmd.Infof("\nConnected to %s.\n\nVault: %s (%s)\n", safeTerminalText(peer.Name), safeTerminalText(info.Name), shortVaultID(info.ID)); err != nil {
		return err
	}
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
	if err := cmd.Infof("Creating %s...\n", safeTerminalText(cloneName)); err != nil {
		return err
	}
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
	if err := cmd.Infof("Receiving items...\n"); err != nil {
		return err
	}
	index := 0
	progressCtx, cancelProgress := context.WithCancel(ctx)
	defer cancelProgress()
	var writeErr error
	execution, err := session.ExecuteWithProgress(progressCtx, peer, plan, func(entry pawsync.PlanEntry, started bool, operationErr error) {
		if writeErr != nil {
			return
		}
		if started {
			index++
			writeErr = cmd.Infof("[%d/%d] %s... ", index, len(plan.Entries), syncItemName(manifestItem(local, remote, entry.ItemID)))
		} else if operationErr != nil {
			writeErr = cmd.Infof("Failed\n")
		} else {
			writeErr = cmd.Infof("Done\n")
		}
		if writeErr != nil {
			cancelProgress()
		}
	})
	if writeErr != nil {
		return writeErr
	}
	if err != nil {
		return fmt.Errorf("clone interrupted after %d of %d items: %w", len(execution.Completed), len(plan.Entries), err)
	}
	if err := cmd.storage.CompleteClone(vault.Accounts[0].ID); err != nil {
		return fmt.Errorf("complete cloned vault: %w", err)
	}
	successful = true
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
