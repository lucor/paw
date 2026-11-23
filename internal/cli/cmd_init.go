// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"

	urfave "github.com/urfave/cli/v3"
)

// InitCmd initializes a vault
type InitCmd struct {
	*command
	vaultName string
}

// newInitCommand constructs the vault-initialization command.
func newInitCommand(base *command) *urfave.Command {
	cmd := &InitCmd{command: base}
	c := &urfave.Command{
		Name:         "init",
		Usage:        "Initialize a vault",
		ArgsUsage:    "NAME",
		Before:       cmd.Validate,
		Action:       cmd.Run,
		OnUsageError: onUsageError,
	}
	return c
}

// Validate checks that a vault name was provided.
func (cmd *InitCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	cmd.vaultName = c.Args().First()
	return ctx, nil
}

// Run initializes a new vault using the supplied command input.
func (cmd *InitCmd) Run(ctx context.Context, _ *urfave.Command) error {
	base := cmd.command
	s := base.storage
	password, err := cmd.prompter.askPasswordWithConfirm()
	if err != nil {
		return err
	}
	_, err = s.CreateVault(cmd.vaultName, password)
	if err != nil {
		return err
	}
	return nil
}
