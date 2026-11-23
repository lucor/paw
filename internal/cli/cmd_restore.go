// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/storage"
)

// RestoreCmd restores a backup into a new vault.
type RestoreCmd struct {
	*command
	backupPath string
	name       string
	yes        bool
}

// newRestoreCommand constructs the vault-restore command.
func newRestoreCommand(base *command) *urfave.Command {
	cmd := &RestoreCmd{command: base}
	c := &urfave.Command{
		Name:      "restore",
		Usage:     "Restore a Paw or CXF backup into a new vault",
		ArgsUsage: "BACKUP",
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "name", Usage: "Name for the restored vault"},
			&urfave.BoolFlag{Name: "yes", Usage: "Skip restore confirmation"},
		},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the backup argument and captures the requested vault name.
func (cmd *RestoreCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	cmd.backupPath = c.Args().First()
	cmd.name = c.String("name")
	cmd.yes = c.Bool("yes")
	return ctx, nil
}

// Run reads a backup, confirms the restore, and creates the vault.
func (cmd *RestoreCmd) Run(ctx context.Context, _ *urfave.Command) error {
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
	defer stop()
	file, err := os.Open(cmd.backupPath)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer file.Close()
	var backup *storage.PawBackup
	if strings.EqualFold(filepath.Ext(cmd.backupPath), ".paw") {
		password, readErr := cmd.prompter.askPassword("Archive password")
		if readErr != nil {
			return readErr
		}
		backup, err = storage.ReadPawArchive(ctx, file, password)
	} else {
		backup, err = storage.ReadPawBackup(ctx, file, "")
		if errors.Is(err, storage.ErrBackupPasswordRequired) {
			password, readErr := cmd.prompter.askPassword("Backup password")
			if readErr != nil {
				return readErr
			}
			if _, err = file.Seek(0, 0); err != nil {
				return err
			}
			backup, err = storage.ReadPawBackup(ctx, file, password)
		}
	}
	if err != nil {
		return err
	}
	defer backup.Close()
	summary := fmt.Sprintf("Vault: %s\nCollections: %d\nCredentials: %d\n", backup.Name, backup.Collections, backup.Credentials)
	if backup.CreationAt != nil {
		summary += fmt.Sprintf("Created: %s\n", time.Unix(int64(*backup.CreationAt), 0).UTC().Format(time.RFC3339))
	}
	if backup.ExportedAt != 0 {
		summary += fmt.Sprintf("CXF document date: %s\n", time.Unix(int64(backup.ExportedAt), 0).UTC().Format(time.RFC3339))
	}
	if err := cmd.Infof("%s", summary); err != nil {
		return err
	}
	name := cmd.name
	if name == "" {
		name, err = cmd.prompter.askWithDefault("New vault name", backup.Name)
		if err != nil {
			return err
		}
	}
	if !cmd.yes {
		confirmed, err := cmd.prompter.askYesNo("Create restored vault?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return errCanceled
		}
	}
	password, err := cmd.prompter.askPassword("New master password")
	if err != nil {
		return err
	}
	confirm, err := cmd.prompter.askPassword("Confirm new master password")
	if err != nil {
		return err
	}
	if password != confirm {
		return fmt.Errorf("master passwords do not match")
	}
	vault, err := cmd.storage.Restore(ctx, backup, name, password)
	if err != nil {
		return err
	}
	if vault.Accounts[0].ID != backup.AccountID {
		if err := cmd.Warnf("The original vault ID is already in use. An independent copy was created.\n"); err != nil {
			return err
		}
	}
	return cmd.Printf("%s\n", storage.FormatVaultID(vault.Accounts[0].ID))
}
