// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
)

// LockCmd implements the command that locks a Paw vault session.
type LockCmd struct {
	*command
	vaultName string
}

// newLockCommand constructs the vault-lock command.
func newLockCommand(base *command) *urfave.Command {
	cmd := &LockCmd{command: base}
	c := &urfave.Command{
		Name:         "lock",
		Usage:        "Locks a vault",
		ArgsUsage:    "VAULT",
		Description:  vaultReferenceHelp,
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks that exactly one vault argument was provided.
func (cmd *LockCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	cmd.vaultName = c.Args().First()
	return ctx, nil
}

// Run invalidates the selected vault's sessions in the agent.
func (cmd *LockCmd) Run(ctx context.Context, _ *urfave.Command) error {
	base := cmd.command
	s := base.storage
	info, err := base.resolveVaultInfo(cmd.vaultName)
	if err != nil {
		return err
	}
	base.vaultID = info.ID
	c, err := base.agentClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}
	err = c.Lock(base.vaultID)
	if err != nil {
		return err
	}

	return c.Lock(base.vaultID)
}
