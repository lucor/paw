// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/model"
)

func newAttachmentCommand(base *command) *urfave.Command {
	return &urfave.Command{Name: "attachment", Usage: "Manage Item attachments", Action: commandGroupAction, Commands: []*urfave.Command{
		{Name: "add", ArgsUsage: "VAULT/TYPE/ITEM PATH", Flags: []urfave.Flag{sessionFlag()}, Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
			cmd, vault, item, err := attachmentItem(base, c)
			if err != nil {
				return err
			}
			file, err := os.Open(c.Args().Get(1))
			if err != nil {
				return fmt.Errorf("open attachment source: %w", err)
			}
			defer file.Close()
			metadata, err := cmd.storage.AddAttachment(vault, item.ID, filepathBase(c.Args().Get(1)), file)
			if err != nil {
				return err
			}
			_, err = fmt.Fprintf(cmd.out, "%s\t%s\t%d bytes\n", metadata.ID, metadata.Name, metadata.DecryptedSize)
			return err
		})},
		{Name: "list", ArgsUsage: "VAULT/TYPE/ITEM", Flags: []urfave.Flag{sessionFlag()}, Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			cmd, vault, item, err := attachmentItem(base, c)
			if err != nil {
				return err
			}
			files, err := cmd.storage.ListAttachments(vault, item.ID)
			if err != nil {
				return err
			}
			for _, file := range files {
				if _, err := fmt.Fprintf(cmd.out, "%s\t%s\t%d bytes\n", file.ID, file.Name, file.DecryptedSize); err != nil {
					return err
				}
			}
			return nil
		})},
		{Name: "save", ArgsUsage: "VAULT/TYPE/ITEM FILE_ID --output PATH", Flags: []urfave.Flag{sessionFlag(), &urfave.StringFlag{Name: "output", Required: true}}, Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
			cmd, vault, item, err := attachmentItem(base, c)
			if err != nil {
				return err
			}
			return cmd.storage.SaveAttachment(vault, item.ID, c.Args().Get(1), c.String("output"))
		})},
		{Name: "remove", ArgsUsage: "VAULT/TYPE/ITEM FILE_ID", Flags: []urfave.Flag{sessionFlag()}, Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
			cmd, vault, item, err := attachmentItem(base, c)
			if err != nil {
				return err
			}
			return cmd.storage.RemoveAttachment(vault, item.ID, c.Args().Get(1))
		})},
	}}
}

func attachmentItem(base *command, c *urfave.Command) (*command, *model.Vault, *model.Item, error) {
	cmd := base.forInvocation(c)
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return nil, nil, nil, err
	}
	key, err := cmd.loadVaultKey(path.vaultName, c.String("session"))
	if err != nil {
		return nil, nil, nil, err
	}
	vault, err := cmd.loadVault(key)
	if err != nil {
		return nil, nil, nil, err
	}
	summary, err := vault.FindItem(path.itemName, path.itemType)
	if err != nil {
		return nil, nil, nil, err
	}
	item, err := cmd.storage.LoadItem(vault, summary.ID)
	return cmd, vault, item, err
}

func filepathBase(path string) string {
	return filepath.Base(path)
}
