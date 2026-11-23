// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
)

// RemoveCmd removes an item from the vault
type RemoveCmd struct {
	*command
	sessionID string
	itemPath
	yes bool
}

// newRemoveCommand constructs the item-removal command.
func newRemoveCommand(base *command) *urfave.Command {
	cmd := &RemoveCmd{command: base}
	c := &urfave.Command{
		Name:        "rm",
		Aliases:     []string{"remove"},
		Usage:       "Removes an item from the vault (alias: remove)",
		ArgsUsage:   "VAULT/ITEM_TYPE/ITEM_NAME",
		Description: vaultReferenceHelp,
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.BoolFlag{Name: "yes", Usage: "Delete without interactive confirmation"},
		},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the item path before the terminal requirement is applied.
func (cmd *RemoveCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return ctx, err
	}
	cmd.itemPath = path
	cmd.sessionID = c.String("session")
	cmd.yes = c.Bool("yes")
	return ctx, nil
}

// Run removes the selected item from its vault.
func (cmd *RemoveCmd) Run(ctx context.Context, _ *urfave.Command) error {
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

	vault, err := base.loadVault(key)
	if err != nil {
		return err
	}

	summary, err := vault.FindItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}

	if !cmd.yes {
		msg := fmt.Sprintf("Are you sure you want to delete %q?", safeTerminalText(cmd.itemPath.String()))
		confirm, err := cmd.prompter.askYesNo(msg, false)
		if err != nil {
			return err
		}
		if !confirm {
			return nil
		}
	}

	err = s.DeleteItem(vault, summary.ID)
	if err != nil {
		return err
	}

	now := base.now().UTC()
	appState.Modified = now
	err = s.StoreAppState(appState)
	if err != nil {
		return err
	}

	return nil
}
