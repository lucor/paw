// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/storage"
)

// ExportCmd exports a vault as a CXF document or Paw archive.
type ExportCmd struct {
	*command
	vaultName   string
	destination string
	sessionID   string
	plaintext   bool
	pawArchive  bool
}

// newExportCommand constructs the vault-export command.
func newExportCommand(base *command) *urfave.Command {
	cmd := &ExportCmd{command: base}
	c := &urfave.Command{
		Name:        "export",
		Usage:       "Export a vault as CXF or an encrypted Paw archive",
		ArgsUsage:   "VAULT DESTINATION",
		Description: vaultReferenceHelp,
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.BoolFlag{Name: "plaintext", Usage: "Export unencrypted CXF after confirmation"},
			&urfave.BoolFlag{Name: "encrypted", Usage: "Export password-encrypted CXF (default)"},
			&urfave.BoolFlag{Name: "paw", Usage: "Export an encrypted Paw archive including attachments"},
		},
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks export arguments and format flags.
func (cmd *ExportCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 2, 2); err != nil {
		return ctx, err
	}
	plaintext, pawArchive := c.Bool("plaintext"), c.Bool("paw")
	if (plaintext && c.Bool("encrypted")) || (pawArchive && (plaintext || c.Bool("encrypted"))) {
		return ctx, fmt.Errorf("choose exactly one export format")
	}
	cmd.vaultName = c.Args().Get(0)
	cmd.destination = c.Args().Get(1)
	cmd.sessionID = c.String("session")
	cmd.plaintext = plaintext
	cmd.pawArchive = pawArchive
	return ctx, nil
}

// Run exports the selected vault after collecting any required passwords.
func (cmd *ExportCmd) Run(ctx context.Context, _ *urfave.Command) error {
	password := ""
	if cmd.plaintext {
		confirmed, err := cmd.prompter.askYesNo(storage.PlaintextExportWarning+" Continue?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return errCanceled
		}
	} else {
		prompt := "Export password"
		if cmd.pawArchive {
			prompt = "Archive password"
		}
		var err error
		password, err = cmd.prompter.askPassword(prompt)
		if err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("export password is required")
		}
		confirmPrompt := "Confirm export password"
		if cmd.pawArchive {
			confirmPrompt = "Confirm archive password"
		}
		confirm, err := cmd.prompter.askPassword(confirmPrompt)
		if err != nil {
			return err
		}
		if password != confirm {
			return fmt.Errorf("export passwords do not match")
		}
	}

	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return err
	}
	vault, err := cmd.loadVault(key)
	if err != nil {
		return err
	}
	if cmd.pawArchive {
		err = storage.ExportPawFile(ctx, cmd.storage, vault, cmd.destination, password)
	} else {
		err = storage.ExportFile(ctx, cmd.storage, vault, cmd.destination, password, cmd.plaintext)
	}
	if err != nil {
		return err
	}
	message := "Credentials exported successfully."
	if cmd.pawArchive {
		message = "Paw archive exported successfully."
	}
	_, err = fmt.Fprintln(cmd.out, message)
	return err
}
