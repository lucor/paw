// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"time"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/paw"
)

const (
	clipboardWriteTimeout = 1 * time.Second
)

// Show shows an item details
type ShowCmd struct {
	*command
	sessionID string
	itemPath
	clipboard bool
}

func newShowCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "show",
		Usage:     "Shows item details",
		ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.BoolFlag{Name: "clip", Aliases: []string{"c"}, Usage: "Copy the secret to the clipboard instead of printing it"},
		},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
			if err != nil {
				return err
			}
			cmd := &ShowCmd{command: base, itemPath: path, sessionID: c.String("session"), clipboard: c.Bool("clip")}
			return cmd.Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *ShowCmd) Run(ctx context.Context) error {
	base := cmd.command
	s := base.storage
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return err
	}

	item, err := paw.NewItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}

	item, err = s.LoadItem(vault, item.GetMetadata())
	if err != nil {
		return err
	}

	var pclip []byte
	var pclipMsg string
	switch cmd.itemType {
	case paw.LoginItemType:
		v := item.(*paw.Login)
		fmt.Fprintf(cmd.out, "URL: %s\n", v.URL)
		fmt.Fprintf(cmd.out, "Username: %s\n", v.Username)
		if !cmd.clipboard {
			fmt.Fprintf(cmd.out, "Password: %s\n", v.Password.Value)
		} else {
			pclip = []byte(v.Password.Value)
			pclipMsg = "[✓] password copied to clipboard"
		}
		if v.Note != nil {
			fmt.Fprintf(cmd.out, "Note: %s\n", v.Note.Value)
		}
	case paw.PasswordItemType:
		v := item.(*paw.Password)
		if !cmd.clipboard {
			fmt.Fprintf(cmd.out, "Password: %s\n", v.Value)
		} else {
			pclip = []byte(v.Value)
			pclipMsg = "[✓] password copied to clipboard"
		}
		if v.Note != nil {
			fmt.Fprintf(cmd.out, "Note: %s\n", v.Note.Value)
		}
	case paw.SSHKeyItemType:
		v := item.(*paw.SSHKey)
		if !cmd.clipboard {
			fmt.Fprintf(cmd.out, "Private key: %s\n", v.PrivateKey)
		} else {
			pclip = []byte(v.PrivateKey)
			pclipMsg = "[✓] private key copied to clipboard"
		}
		if v.Passphrase != nil {
			fmt.Fprintf(cmd.out, "Passphrase: %s\n", v.Passphrase.Value)
		}
		fmt.Fprintf(cmd.out, "Public key: %s\n", v.PublicKey)
		fmt.Fprintf(cmd.out, "Fingerprint: %s\n", v.Fingerprint)
		addToAgent := "No"
		if v.AddToAgent {
			addToAgent = "Yes"
		}
		fmt.Fprintf(cmd.out, "Add to agent: %s\n", addToAgent)
		if v.Note != nil {
			fmt.Fprintf(cmd.out, "Note: %s\n", v.Note.Value)
		}
	case paw.NoteItemType:
		v := item.(*paw.Note)
		fmt.Fprintf(cmd.out, "Note: %s\n", v.Value)
	}

	fmt.Fprintf(cmd.out, "Modified: %s\n", item.GetMetadata().Modified.Local().Format(time.RFC1123))
	fmt.Fprintf(cmd.out, "Created: %s\n", item.GetMetadata().Created.Local().Format(time.RFC1123))

	if pclip != nil {
		ctx, cancel := context.WithTimeout(ctx, clipboardWriteTimeout)
		defer cancel()
		err := base.clipboard(ctx, pclip)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.out, pclipMsg)
	}
	return nil
}
