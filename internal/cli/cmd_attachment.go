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

const (
	attachmentAdd    = "add"
	attachmentList   = "list"
	attachmentSave   = "save"
	attachmentRemove = "remove"
)

// AttachmentCmd manages files attached to a vault item.
type AttachmentCmd struct {
	*command
	operation string
	itemPath
	sessionID string
	filePath  string
	fileID    string
	output    string
}

// newAttachmentCommand constructs the attachment command group.
func newAttachmentCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:   "attachment",
		Usage:  "Manage Item attachments",
		Action: commandGroupAction,
		Commands: []*urfave.Command{
			newAttachmentSubcommand(base, attachmentAdd, "VAULT/TYPE/ITEM PATH", false),
			newAttachmentSubcommand(base, attachmentList, "VAULT/TYPE/ITEM", false),
			newAttachmentSubcommand(base, attachmentSave, "VAULT/TYPE/ITEM FILE_ID --output PATH", true),
			newAttachmentSubcommand(base, attachmentRemove, "VAULT/TYPE/ITEM FILE_ID", false),
		},
		OnUsageError: onUsageError,
	}
}

// newAttachmentSubcommand builds one attachment operation command.
func newAttachmentSubcommand(base *command, operation, argsUsage string, outputRequired bool) *urfave.Command {
	cmd := &AttachmentCmd{command: base, operation: operation}
	flags := []urfave.Flag{sessionFlag()}
	if outputRequired {
		flags = append(flags, &urfave.StringFlag{Name: "output", Required: true})
	}
	c := &urfave.Command{
		OnUsageError: onUsageError,
		Name:         operation,
		ArgsUsage:    argsUsage,
		Description:  vaultReferenceHelp,
		Flags:        flags,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the item path and arguments for the attachment operation.
func (cmd *AttachmentCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	argCount := 2
	if cmd.operation == attachmentList {
		argCount = 1
	}
	if err := validateArgs(c, argCount, argCount); err != nil {
		return ctx, err
	}
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return ctx, err
	}
	cmd.itemPath = path
	cmd.sessionID = c.String("session")
	cmd.filePath = c.Args().Get(1)
	cmd.fileID = c.Args().Get(1)
	cmd.output = c.String("output")
	return ctx, nil
}

// Run performs the selected attachment operation.
func (cmd *AttachmentCmd) Run(ctx context.Context, _ *urfave.Command) error {
	vault, item, err := cmd.loadItem()
	if err != nil {
		return err
	}
	switch cmd.operation {
	case attachmentAdd:
		file, err := os.Open(cmd.filePath)
		if err != nil {
			return fmt.Errorf("open attachment source: %w", err)
		}
		defer file.Close()
		metadata, err := cmd.storage.AddAttachment(vault, item.ID, filepath.Base(cmd.filePath), file)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.out, "%s\t%s\t%d bytes\n", metadata.ID, metadata.Name, metadata.DecryptedSize)
		return err
	case attachmentList:
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
	case attachmentSave:
		return cmd.storage.SaveAttachment(vault, item.ID, cmd.fileID, cmd.output)
	case attachmentRemove:
		return cmd.storage.RemoveAttachment(vault, item.ID, cmd.fileID)
	default:
		return fmt.Errorf("unknown attachment operation %q", cmd.operation)
	}
}

// loadItem unlocks the vault and loads the addressed item.
func (cmd *AttachmentCmd) loadItem() (*model.Vault, *model.Item, error) {
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return nil, nil, err
	}
	vault, err := cmd.loadVault(key)
	if err != nil {
		return nil, nil, err
	}
	summary, err := vault.FindItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return nil, nil, err
	}
	item, err := cmd.storage.LoadItem(vault, summary.ID)
	return vault, item, err
}
