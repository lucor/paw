// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

// ExportCmd exports a vault as a CXF document or Paw archive.
type ExportCmd struct {
	*command
	vaultName     string
	destination   string
	sessionID     string
	plaintext     bool
	pawArchive    bool
	passwordStdin bool
	yes           bool
	input         io.Reader
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
			&urfave.BoolFlag{Name: "password-stdin", Usage: "Read the export password from stdin"},
			&urfave.BoolFlag{Name: "yes", Usage: "Skip plaintext export confirmation"},
			&urfave.BoolFlag{Name: "paw", Usage: "Export an encrypted Paw archive including attachments"},
		},
		OnUsageError: onUsageError,
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
	plaintext := c.Bool("plaintext")
	pawArchive := c.Bool("paw")
	passwordStdin := c.Bool("password-stdin")
	yes := c.Bool("yes")
	if plaintext && pawArchive {
		return ctx, invalidArgumentsError("--plaintext cannot be combined with --paw")
	}
	if plaintext && passwordStdin {
		return ctx, invalidArgumentsError("--password-stdin cannot be combined with --plaintext")
	}
	if yes && !plaintext {
		return ctx, invalidArgumentsError("--yes is only valid with --plaintext")
	}
	cmd.vaultName = c.Args().Get(0)
	cmd.destination = c.Args().Get(1)
	cmd.sessionID = c.String("session")
	cmd.plaintext = plaintext
	cmd.pawArchive = pawArchive
	cmd.passwordStdin = passwordStdin
	cmd.yes = yes
	if passwordStdin {
		cmd.input = c.Reader
	}
	return ctx, nil
}

// Run exports the selected vault after collecting any required passwords.
func (cmd *ExportCmd) Run(ctx context.Context, _ *urfave.Command) error {
	password := ""
	var vaultLoaded bool
	var vaultErr error
	var vault *model.Vault
	if cmd.passwordStdin {
		vault, vaultErr = cmd.loadExportVault()
		if vaultErr != nil {
			return vaultErr
		}
		vaultLoaded = true
	}
	if cmd.plaintext {
		if !cmd.yes {
			confirmed, err := cmd.prompter.askYesNo(storage.PlaintextExportWarning+" Continue?", false)
			if err != nil {
				return err
			}
			if !confirmed {
				return errCanceled
			}
		}
	} else {
		prompt := "Export password"
		if cmd.pawArchive {
			prompt = "Archive password"
		}
		var err error
		if cmd.passwordStdin {
			password, err = cmd.readExportPassword()
		} else {
			password, err = cmd.prompter.askPassword(prompt)
		}
		if err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("export password is required")
		}
		if !cmd.passwordStdin {
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
	}

	if !vaultLoaded {
		vault, vaultErr = cmd.loadExportVault()
		if vaultErr != nil {
			return vaultErr
		}
	}
	var err error
	if cmd.pawArchive {
		err = storage.ExportPawFile(ctx, cmd.storage, vault, cmd.destination, password)
	} else {
		err = storage.ExportFile(ctx, cmd.storage, vault, cmd.destination, password, cmd.plaintext)
	}
	if err != nil {
		return err
	}
	return nil
}

// loadExportVault resolves and decrypts the selected vault using its existing session.
func (cmd *ExportCmd) loadExportVault() (*model.Vault, error) {
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return nil, err
	}
	return cmd.loadVault(key)
}

// readExportPassword reads exactly one password line from the selected input stream.
func (cmd *ExportCmd) readExportPassword() (string, error) {
	if cmd.input == nil {
		return "", fmt.Errorf("export password input is unavailable")
	}
	password, err := bufio.NewReader(singleByteReader{cmd.input}).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read export password from stdin: %w", err)
	}
	password = strings.TrimSuffix(password, "\n")
	password = strings.TrimSuffix(password, "\r")
	if password == "" {
		return "", fmt.Errorf("export password is required")
	}
	return password, nil
}
