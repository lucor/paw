// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

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
	name      string
	input     io.Reader
}

// newAttachmentCommand constructs the attachment command group.
func newAttachmentCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:   "attachment",
		Usage:  "Manage item attachments",
		Action: commandGroupAction,
		Commands: []*urfave.Command{
			newAttachmentSubcommand(base, attachmentAdd, "Add an attachment", "VAULT/TYPE/ITEM FILE", false),
			newAttachmentSubcommand(base, attachmentList, "List item attachments", "VAULT/TYPE/ITEM", false),
			newAttachmentSubcommand(base, attachmentSave, "Save an attachment", "VAULT/TYPE/ITEM ID --output FILE", true),
			newAttachmentSubcommand(base, attachmentRemove, "Remove an attachment", "VAULT/TYPE/ITEM ID", false),
		},
		OnUsageError: onUsageError,
	}
}

// newAttachmentSubcommand builds one attachment operation command.
func newAttachmentSubcommand(base *command, operation, usage, argsUsage string, outputRequired bool) *urfave.Command {
	cmd := &AttachmentCmd{command: base, operation: operation}
	flags := []urfave.Flag{sessionFlag()}
	if operation == attachmentAdd {
		flags = append(flags, &urfave.StringFlag{Name: "name", Usage: "Attachment name when reading from stdin"})
	}
	if outputRequired {
		flags = append(flags, &urfave.StringFlag{Name: "output", Required: true, Usage: "Write the attachment to FILE"})
	}
	c := &urfave.Command{
		OnUsageError: onUsageError,
		Name:         operation,
		Usage:        usage,
		ArgsUsage:    argsUsage,
		Description:  vaultReferenceHelp + " Use - as FILE to read from stdin when adding. Set --name when reading from stdin. Saving requires --output FILE.",
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
	cmd.name = c.String("name")
	if cmd.operation == attachmentAdd {
		if cmd.filePath == "-" {
			if strings.TrimSpace(cmd.name) == "" {
				return ctx, invalidArgumentsError("--name is required when FILE is -")
			}
			cmd.input = c.Reader
		} else if c.IsSet("name") {
			return ctx, invalidArgumentsError("--name is only supported when FILE is -")
		}
	}
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
		name := cmd.name
		source := cmd.input
		if cmd.filePath != "-" {
			file, err := os.Open(cmd.filePath)
			if err != nil {
				return fmt.Errorf("open attachment source: %w", err)
			}
			defer file.Close()
			source = file
			name = filepath.Base(cmd.filePath)
		}
		metadata, err := cmd.storage.AddAttachment(vault, item.ID, name, source)
		if err != nil {
			return err
		}
		return cmd.Printf("%s\n", metadata.ID)
	case attachmentList:
		files, err := cmd.storage.ListAttachments(vault, item.ID)
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		writer := tabwriter.NewWriter(cmd.Stdout(), 0, 0, 1, ' ', 0)
		if _, err := fmt.Fprintln(writer, "ID\tName\tSize"); err != nil {
			return err
		}
		for _, file := range files {
			if _, err := fmt.Fprintf(writer, "%s\t%s\t%d bytes\n", file.ID, safeTerminalText(file.Name), file.DecryptedSize); err != nil {
				return err
			}
		}
		return writer.Flush()
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
