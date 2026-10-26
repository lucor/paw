// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
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

	vault, err := base.loadVault(key)
	if err != nil {
		return err
	}

	summary, err := findVaultItem(vault, cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}
	item, err := s.LoadItem(vault, summary.ID)
	if err != nil {
		return err
	}

	var pclip []byte
	var pclipMsg string
	switch cmd.itemType {
	case paw.LoginItemType:
		if item.Scope != nil && len(item.Scope.URLs) > 0 {
			fmt.Fprintf(cmd.out, "URL: %s\n", item.Scope.URLs[0])
		}
		credential, _, err := basicAuth(item)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.out, "Username: %s\n", usernameValue(credential))
		if !cmd.clipboard {
			fmt.Fprintf(cmd.out, "Password: %s\n", passwordValue(credential))
		} else {
			pclip = []byte(passwordValue(credential))
			pclipMsg = "[✓] password copied to clipboard"
		}
		if note, _, err := noteCredential(item); err == nil && note.Content.Value != "" {
			fmt.Fprintf(cmd.out, "Note: %s\n", note.Content.Value)
		}
		for _, raw := range item.Credentials {
			typeName, err := cxf.CredentialTypeOf(raw)
			if err != nil {
				return err
			}
			if typeName == cxf.CredentialTypeTOTP {
				fmt.Fprintln(cmd.out, "TOTP: configured")
				break
			}
		}
	case paw.PasswordItemType:
		credential, _, err := basicAuth(item)
		if err != nil {
			return err
		}
		if !cmd.clipboard {
			fmt.Fprintf(cmd.out, "Password: %s\n", passwordValue(credential))
		} else {
			pclip = []byte(passwordValue(credential))
			pclipMsg = "[✓] password copied to clipboard"
		}
	case paw.SSHKeyItemType:
		var credential cxf.SSHKey
		for _, raw := range item.Credentials {
			typeName, err := cxf.CredentialTypeOf(raw)
			if err != nil {
				return err
			}
			if typeName == cxf.CredentialTypeSSHKey {
				if err := json.Unmarshal(raw, &credential); err != nil {
					return err
				}
				break
			}
		}
		privateKey, publicKey, fingerprint, err := sshKeyFromCXF(credential)
		if err != nil {
			return err
		}
		if !cmd.clipboard {
			fmt.Fprintf(cmd.out, "Private key: %s\n", privateKey)
			fmt.Fprintf(cmd.out, "Public key: %s", publicKey)
		} else {
			pclip = []byte(privateKey)
			pclipMsg = "[✓] private key copied to clipboard"
		}
		fmt.Fprintf(cmd.out, "Fingerprint: %s\n", fingerprint)
		addToAgent := "No"
		if item.Metadata.AddToAgent {
			addToAgent = "Yes"
		}
		fmt.Fprintf(cmd.out, "Add to agent: %s\n", addToAgent)
	case paw.NoteItemType:
		note, _, err := noteCredential(item)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.out, "Note: %s\n", note.Content.Value)
	}

	if item.ModifiedAt != nil {
		fmt.Fprintf(cmd.out, "Modified: %s\n", time.Unix(int64(*item.ModifiedAt), 0).Local().Format(time.RFC1123))
	}
	if item.CreationAt != nil {
		fmt.Fprintf(cmd.out, "Created: %s\n", time.Unix(int64(*item.CreationAt), 0).Local().Format(time.RFC1123))
	}

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
