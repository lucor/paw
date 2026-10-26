// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
)

// Init initializes a vault
type InitCmd struct {
	*command
	vaultName string
}

func newInitCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "init",
		Usage:     "Initializes a vault",
		ArgsUsage: "VAULT",
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			return (&InitCmd{command: base, vaultName: c.Args().First()}).Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *InitCmd) Run(ctx context.Context) error {
	base := cmd.command
	s := base.storage
	fmt.Fprintf(cmd.out, "Initializing vault %q\n", cmd.vaultName)
	password, err := cmd.prompter.askPasswordWithConfirm()
	if err != nil {
		return err
	}
	_, err = s.CreateVault(cmd.vaultName, password)
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] vault %q created\n", cmd.vaultName)
	return nil
}
