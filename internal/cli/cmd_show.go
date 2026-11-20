// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"strings"
	"time"
	"unicode"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
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
	reveal    bool
	field     string
}

// newShowCommand constructs the item-display command.
func newShowCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:        "show",
		Usage:       "Shows item details with secrets masked by default",
		ArgsUsage:   "VAULT/ITEM_TYPE/ITEM_NAME",
		Description: vaultReferenceHelp,
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.BoolFlag{
				Name:    "clip",
				Aliases: []string{"c"},
				Usage:   "Copy the default secret, or the selected field, to the clipboard",
			},
			&urfave.BoolFlag{
				Name:  "reveal",
				Usage: "Display secret values in cleartext",
			},
			&urfave.StringFlag{
				Name:  "field",
				Usage: "Display only the named field value",
			},
		},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
			if err != nil {
				return err
			}
			if c.IsSet("field") && c.String("field") == "" {
				return fmt.Errorf("--field requires a field name")
			}
			if c.Bool("clip") && c.Bool("reveal") {
				return fmt.Errorf("--clip cannot be combined with --reveal")
			}
			cmd := &ShowCmd{command: base, itemPath: path, sessionID: c.String("session"), clipboard: c.Bool("clip"), reveal: c.Bool("reveal"), field: c.String("field")}
			return cmd.Run(ctx)
		}),
	}
}

// Run displays the selected item from its vault.
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

	summary, err := vault.FindItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}
	item, err := s.LoadItem(vault, summary.ID)
	if err != nil {
		return err
	}
	return cmd.showItem(ctx, item)
}

// showItem writes a readable representation of an item to the command output.
func (cmd *ShowCmd) showItem(ctx context.Context, item *model.Item) error {
	if cmd.field != "" {
		value, err := itemField(item, cmd.itemType, cmd.field)
		if err != nil {
			return err
		}
		if cmd.clipboard {
			if err := cmd.copyValue(ctx, value); err != nil {
				return err
			}
			fmt.Fprintln(cmd.errOut, "[✓] field copied to clipboard")
			return nil
		}
		_, err = fmt.Fprint(cmd.out, value)
		return err
	}
	if cmd.clipboard {
		value, label, err := itemDefaultSecret(item, cmd.itemType)
		if err != nil {
			return err
		}
		if err := cmd.copyValue(ctx, value); err != nil {
			return err
		}
		fmt.Fprintf(cmd.errOut, "[✓] %s copied to clipboard\n", label)
		return nil
	}
	switch cmd.itemType {
	case model.LoginItemType:
		if item.Scope != nil {
			for _, url := range item.Scope.URLs {
				fmt.Fprintf(cmd.out, "URL: %s\n", safeTerminalText(url))
			}
		}
		credential, _, err := item.BasicAuth()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.out, "Username: %s\n", safeTerminalText(usernameValue(credential)))
		fmt.Fprintf(cmd.out, "Password: %s\n", safeTerminalText(secretValue(passwordValue(credential), cmd.reveal)))
		if note, _, err := item.Note(); err == nil && note.Content.Value != "" {
			fmt.Fprintf(cmd.out, "Note: %s\n", safeTerminalText(note.Content.Value))
		}
		for _, raw := range item.Credentials {
			typeName, err := cxf.CredentialTypeOf(raw)
			if err != nil {
				return err
			}
			if typeName == cxf.CredentialTypeTOTP {
				var totp cxf.TOTP
				if err := json.Unmarshal(raw, &totp); err != nil {
					return err
				}
				fmt.Fprintln(cmd.out, "TOTP: configured")
				fmt.Fprintf(cmd.out, "TOTP secret: %s\n", safeTerminalText(secretValue(totp.Secret, cmd.reveal)))
				fmt.Fprintf(cmd.out, "TOTP algorithm: %s\n", safeTerminalText(totp.Algorithm))
				fmt.Fprintf(cmd.out, "TOTP digits: %d\n", totp.Digits)
				fmt.Fprintf(cmd.out, "TOTP period: %d seconds\n", totp.Period)
				if totp.Username != "" {
					fmt.Fprintf(cmd.out, "TOTP username: %s\n", safeTerminalText(totp.Username))
				}
				if totp.Issuer != "" {
					fmt.Fprintf(cmd.out, "TOTP issuer: %s\n", safeTerminalText(totp.Issuer))
				}
			}
		}
	case model.PasswordItemType:
		credential, _, err := item.BasicAuth()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.out, "Password: %s\n", safeTerminalText(secretValue(passwordValue(credential), cmd.reveal)))
	case model.SSHKeyItemType:
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
		privateKey, publicKey, fingerprint, err := model.FormatSSHKey(credential)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.out, "Private key: %s\n", safeTerminalText(secretValue(privateKey, cmd.reveal)))
		fmt.Fprintf(cmd.out, "Public key: %s", safeTerminalText(publicKey))
		fmt.Fprintf(cmd.out, "Fingerprint: %s\n", safeTerminalText(fingerprint))
		if credential.CreationDate != nil {
			fmt.Fprintf(cmd.out, "Key creation date: %s\n", safeTerminalText(editableDisplayValue(credential.CreationDate, cmd.reveal)))
		}
		if credential.ExpiryDate != nil {
			fmt.Fprintf(cmd.out, "Key expiry date: %s\n", safeTerminalText(editableDisplayValue(credential.ExpiryDate, cmd.reveal)))
		}
		if credential.KeyGenerationSource != nil {
			fmt.Fprintf(cmd.out, "Key generation source: %s\n", safeTerminalText(editableDisplayValue(credential.KeyGenerationSource, cmd.reveal)))
		}
		if credential, _, err := item.BasicAuth(); err == nil && credential.Password != nil && credential.Password.Value != "" {
			fmt.Fprintf(cmd.out, "Passphrase: %s\n", safeTerminalText(secretValue(credential.Password.Value, cmd.reveal)))
		}
		addToAgent := "No"
		if item.Metadata.AddToAgent {
			addToAgent = "Yes"
		}
		fmt.Fprintf(cmd.out, "Add to agent: %s\n", addToAgent)
	case model.NoteItemType:
		note, _, err := item.Note()
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.out, "Note: %s\n", safeTerminalText(note.Content.Value))
	}
	groups, err := item.CustomFields()
	if err != nil {
		return err
	}
	if len(groups) > 0 {
		fmt.Fprintln(cmd.out, "\nCustom fields:")
		for _, group := range groups {
			if group.Credential.Label != "" {
				fmt.Fprintf(cmd.out, "  [%s]\n", safeTerminalText(group.Credential.Label))
			}
			for _, field := range group.Credential.Fields {
				label := field.Label
				if label == "" {
					label = "Field"
				}
				value := field.Value
				if field.FieldType == cxf.FieldTypeConcealedString && !cmd.reveal {
					value = "********"
				}
				fmt.Fprintf(cmd.out, "  %s: %s\n", safeTerminalText(label), safeTerminalText(value))
			}
		}
	}
	if len(item.Tags) > 0 {
		fmt.Fprintf(cmd.out, "Tags: %s\n", safeTerminalText(strings.Join(item.Tags, ", ")))
	}

	if item.ModifiedAt != nil {
		fmt.Fprintf(cmd.out, "Modified: %s\n", time.Unix(int64(*item.ModifiedAt), 0).Local().Format(time.RFC1123))
	}
	if item.CreationAt != nil {
		fmt.Fprintf(cmd.out, "Created: %s\n", time.Unix(int64(*item.CreationAt), 0).Local().Format(time.RFC1123))
	}
	files, err := item.Files()
	if err != nil {
		return fmt.Errorf("cannot read attachment metadata: %w", err)
	}
	if len(files) > 0 {
		fmt.Fprintf(cmd.out, "\nAttachments: %d\n", len(files))
	}
	history, err := item.History()
	if err != nil {
		return fmt.Errorf("cannot read item history: %w", err)
	}
	if len(history.Entries) > 0 {
		if len(files) == 0 {
			fmt.Fprintln(cmd.out)
		}
		fmt.Fprintf(cmd.out, "History: %d versions\n", len(history.Entries))
	}

	return nil
}

func secretValue(value string, reveal bool) string {
	if reveal {
		return value
	}
	return "********"
}

func editableDisplayValue(field *cxf.EditableField, reveal bool) string {
	if field == nil {
		return ""
	}
	if field.FieldType == cxf.FieldTypeConcealedString && !reveal {
		return "********"
	}
	return field.Value
}

func safeTerminalText(value string) string {
	var safe strings.Builder
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			fmt.Fprintf(&safe, "\\u%04x", r)
			continue
		}
		safe.WriteRune(r)
	}
	return safe.String()
}

func (cmd *ShowCmd) copyValue(ctx context.Context, value string) error {
	ctx, cancel := context.WithTimeout(ctx, clipboardWriteTimeout)
	defer cancel()
	return cmd.command.clipboard(ctx, []byte(value))
}

func itemDefaultSecret(item *model.Item, itemType model.ItemType) (string, string, error) {
	switch itemType {
	case model.LoginItemType, model.PasswordItemType:
		credential, _, err := item.BasicAuth()
		if err == nil && credential.Password == nil {
			return "", "", fmt.Errorf("item has no default copyable secret")
		}
		return passwordValue(credential), "password", err
	case model.SSHKeyItemType:
		credential, err := item.SSHKey()
		if err != nil {
			return "", "", err
		}
		privateKey, _, _, err := model.FormatSSHKey(credential)
		return privateKey, "private key", err
	default:
		return "", "", fmt.Errorf("item has no default copyable secret")
	}
}

func itemField(item *model.Item, itemType model.ItemType, name string) (string, error) {
	var values []string
	add := func(label, value string) {
		if strings.EqualFold(label, name) {
			values = append(values, value)
		}
	}
	switch itemType {
	case model.LoginItemType, model.PasswordItemType:
		credential, _, err := item.BasicAuth()
		if err != nil {
			return "", err
		}
		if itemType == model.LoginItemType {
			if credential.Username != nil {
				add("username", credential.Username.Value)
			}
			if item.Scope != nil {
				for _, url := range item.Scope.URLs {
					add("url", url)
				}
			}
		}
		if credential.Password != nil {
			add("password", credential.Password.Value)
		}
	case model.NoteItemType:
		note, index, err := item.Note()
		if err != nil {
			return "", err
		}
		if index >= 0 {
			add("note", note.Content.Value)
		}
	case model.SSHKeyItemType:
		key, err := item.SSHKey()
		if err != nil {
			return "", err
		}
		privateKey, publicKey, _, err := model.FormatSSHKey(key)
		if err != nil {
			return "", err
		}
		add("private key", privateKey)
		add("public key", publicKey)
		if key.CreationDate != nil {
			add("key creation date", key.CreationDate.Value)
		}
		if key.ExpiryDate != nil {
			add("key expiry date", key.ExpiryDate.Value)
		}
		if key.KeyGenerationSource != nil {
			add("key generation source", key.KeyGenerationSource.Value)
		}
		if credential, _, err := item.BasicAuth(); err == nil && credential.Password != nil {
			add("password", credential.Password.Value)
			add("passphrase", credential.Password.Value)
		}
	}
	for _, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return "", err
		}
		if typeName != cxf.CredentialTypeTOTP {
			continue
		}
		var totp cxf.TOTP
		if err := json.Unmarshal(raw, &totp); err != nil {
			return "", err
		}
		add("TOTP secret", totp.Secret)
	}
	groups, err := item.CustomFields()
	if err != nil {
		return "", err
	}
	for _, group := range groups {
		for _, field := range group.Credential.Fields {
			label := field.Label
			if strings.Contains(name, "/") && group.Credential.Label != "" {
				label = group.Credential.Label + "/" + label
			}
			add(label, field.Value)
		}
	}
	if len(values) == 0 {
		return "", fmt.Errorf("field %q was not found", name)
	}
	if len(values) > 1 {
		return "", fmt.Errorf("field %q is ambiguous; use Section/Field", name)
	}
	return values[0], nil
}
