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
		Name: "export", Usage: "Export a vault as CXF or an encrypted Paw archive", ArgsUsage: "VAULT DESTINATION",
		Flags: []urfave.Flag{sessionFlag(), &urfave.BoolFlag{Name: "plaintext", Usage: "Export unencrypted CXF after confirmation"}, &urfave.BoolFlag{Name: "encrypted", Usage: "Export password-encrypted CXF (default)"}, &urfave.BoolFlag{Name: "paw", Usage: "Export an encrypted Paw archive including attachments"}},
		Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			plaintext := c.Bool("plaintext")
			pawArchive := c.Bool("paw")
			if (plaintext && c.Bool("encrypted")) || (pawArchive && (plaintext || c.Bool("encrypted"))) {
				return fmt.Errorf("choose exactly one export format")
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
				prompt := "Export password"
				if pawArchive {
					prompt = "Archive password"
				}
				password, err = base.prompter.askPassword(prompt)
				if err != nil {
					return err
				}
				if password == "" {
					return fmt.Errorf("export password is required")
				}
				confirmPrompt := "Confirm export password"
				if pawArchive {
					confirmPrompt = "Confirm archive password"
				}
				confirm, err := base.prompter.askPassword(confirmPrompt)
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
			var exportErr error
			if pawArchive {
				exportErr = storage.ExportPawFile(ctx, base.storage, vault, c.Args().Get(1), password)
			} else {
				exportErr = storage.ExportFile(ctx, base.storage, vault, c.Args().Get(1), password, plaintext)
			}
			if exportErr != nil {
				return exportErr
			}
			message := "Credentials exported successfully."
			if pawArchive {
				message = "Paw archive exported successfully."
			}
			_, err = fmt.Fprintln(base.out, message)
			return err
		}),
	}
}
