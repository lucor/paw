// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/storage"
)

func newRestoreCommand(base *command) *urfave.Command {
	return &urfave.Command{Name: "restore", Usage: "Restore a Paw CXF backup into a new vault", ArgsUsage: "BACKUP", Flags: []urfave.Flag{&urfave.StringFlag{Name: "name", Usage: "Name for the restored vault"}}, Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
		base := base.forInvocation(c)
		ctx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()
		file, err := os.Open(c.Args().First())
		if err != nil {
			return fmt.Errorf("open backup: %w", err)
		}
		defer file.Close()
		backup, err := storage.ReadPawBackup(ctx, file, "")
		if errors.Is(err, storage.ErrBackupPasswordRequired) {
			password, readErr := base.prompter.askPassword("Backup password")
			if readErr != nil {
				return readErr
			}
			if _, err = file.Seek(0, 0); err != nil {
				return err
			}
			backup, err = storage.ReadPawBackup(ctx, file, password)
		}
		if err != nil {
			return err
		}
		fmt.Fprintf(base.out, "Vault: %s\nCollections: %d\nCredentials: %d\n", backup.Name, backup.Collections, backup.Credentials)
		if backup.CreationAt != nil {
			fmt.Fprintf(base.out, "Created: %s\n", time.Unix(int64(*backup.CreationAt), 0).UTC().Format(time.RFC3339))
		}
		if backup.ExportedAt != 0 {
			fmt.Fprintf(base.out, "CXF document date: %s\n", time.Unix(int64(backup.ExportedAt), 0).UTC().Format(time.RFC3339))
		}
		name := c.String("name")
		if name == "" {
			name, err = base.prompter.askWithDefault("New vault name", backup.Name)
			if err != nil {
				return err
			}
		}
		confirmed, err := base.prompter.askYesNo("Create restored vault?", false)
		if err != nil {
			return err
		}
		if !confirmed {
			return errCanceled
		}
		password, err := base.prompter.askPassword("New master password")
		if err != nil {
			return err
		}
		confirm, err := base.prompter.askPassword("Confirm new master password")
		if err != nil {
			return err
		}
		if password != confirm {
			return fmt.Errorf("master passwords do not match")
		}
		vault, err := base.storage.Restore(ctx, backup, name, password)
		if err != nil {
			return err
		}
		if vault.Accounts[0].ID != backup.AccountID {
			fmt.Fprintln(base.out, "The original vault ID is already in use. An independent copy was created.")
		}
		_, err = fmt.Fprintf(base.out, "Vault restored successfully: %s (%s)\n", name, vault.Accounts[0].ID)
		return err
	})}
}
