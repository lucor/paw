// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/storage"
)

func newExportCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name: "export", Usage: "Export a vault as CXF (encrypted by default)", ArgsUsage: "VAULT DESTINATION",
		Flags: []urfave.Flag{sessionFlag(), &urfave.BoolFlag{Name: "plaintext", Usage: "Export unencrypted CXF after confirmation"}, &urfave.BoolFlag{Name: "encrypted", Usage: "Export password-encrypted CXF (default)"}},
		Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			plaintext := c.Bool("plaintext")
			if plaintext && c.Bool("encrypted") {
				return fmt.Errorf("choose either plaintext or encrypted export")
			}
			password := ""
			if plaintext {
				confirmed, err := base.prompter.askYesNo(storage.PlaintextExportWarning+" Continue?", false)
				if err != nil {
					return err
				}
				if !confirmed {
					return errCanceled
				}
			} else {
				var err error
				password, err = base.prompter.askPassword("Export password")
				if err != nil {
					return err
				}
				if password == "" {
					return fmt.Errorf("export password is required")
				}
				confirm, err := base.prompter.askPassword("Confirm export password")
				if err != nil {
					return err
				}
				if password != confirm {
					return fmt.Errorf("export passwords do not match")
				}
			}
			key, err := base.loadVaultKey(c.Args().Get(0), c.String("session"))
			if err != nil {
				return err
			}
			vault, err := base.loadVault(key)
			if err != nil {
				return err
			}
			if err := storage.ExportFile(ctx, base.storage, vault, c.Args().Get(1), password, plaintext); err != nil {
				return err
			}
			_, err = fmt.Fprintln(base.out, "Credentials exported successfully.")
			return err
		}),
	}
}
