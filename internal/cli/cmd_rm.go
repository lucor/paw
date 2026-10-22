// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/paw"
)

// RemoveCmd removes an item from the vault
type RemoveCmd struct {
	*command
	sessionID string
	itemPath
}

func newRemoveCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "rm",
		Usage:     "Removes an item from the vault",
		ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags:     []urfave.Flag{sessionFlag()},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
			if err != nil {
				return err
			}
			return (&RemoveCmd{command: base, itemPath: path, sessionID: c.String("session")}).Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *RemoveCmd) Run(ctx context.Context) error {
	base := cmd.command
	s := base.storage
	appState, err := s.LoadAppState()
	if err != nil {
		return err
	}

	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return err
	}

	item, err := paw.NewItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}

	if ok := vault.HasItem(item); !ok {
		return fmt.Errorf("item does not exists")
	}

	msg := fmt.Sprintf("Are you sure you want to delete %q?", cmd.itemPath)
	confirm, err := cmd.prompter.askYesNo(msg, false)
	if err != nil {
		return err
	}
	if !confirm {
		return nil
	}

	err = s.DeleteItem(vault, item)
	if err != nil {
		return err
	}

	vault.DeleteItem(item)

	now := base.now().UTC()
	vault.Modified = now
	err = s.StoreVault(vault)
	if err != nil {
		return err
	}

	appState.Modified = now
	err = s.StoreAppState(appState)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.out, "[✓] item %q removed\n", cmd.itemName)
	return nil
}
