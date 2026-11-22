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
	"go.lucor.dev/paw/internal/otp"
	"go.lucor.dev/paw/internal/paw"
)

const (
	clipboardWriteTimeout = 1 * time.Second
)

// ShowCmd shows an item details
type ShowCmd struct {
	*command
	sessionID string
	itemPath
	clipboard bool
	reveal    bool
	field     string
	json      bool
}

// newShowCommand constructs the item-display command.
func newShowCommand(base *command) *urfave.Command {
	cmd := &ShowCmd{command: base}
	c := &urfave.Command{
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
			&urfave.BoolFlag{
				Name:  "json",
				Usage: "Display item details as JSON, including secrets",
			},
			&urfave.BoolFlag{
				Name:  "username",
				Usage: "Display only the username",
			},
			&urfave.BoolFlag{
				Name:  "password",
				Usage: "Display only the password",
			},
			&urfave.BoolFlag{
				Name:  "totp",
				Usage: "Generate and display the current TOTP code",
			},
			&urfave.StringFlag{
				Name:  "field",
				Usage: "Display only the named field value",
			},
		},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the item path and display flags.
func (cmd *ShowCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return ctx, err
	}
	if c.IsSet("field") && c.String("field") == "" {
		return ctx, invalidArgumentsError("--field requires a field name")
	}
	selectors := make([]string, 0, 4)
	for _, name := range []string{"username", "password", "totp"} {
		if c.Bool(name) {
			selectors = append(selectors, name)
		}
	}
	if c.IsSet("field") {
		selectors = append(selectors, c.String("field"))
	}
	if len(selectors) > 1 {
		return ctx, invalidArgumentsError("select only one field")
	}
	if c.Bool("reveal") && len(selectors) != 0 {
		return ctx, invalidArgumentsError("--reveal cannot be combined with a field selector")
	}
	if c.Bool("json") && c.Bool("clip") {
		return ctx, invalidArgumentsError("--json cannot be combined with --clip")
	}
	if c.Bool("clip") && c.Bool("reveal") {
		return ctx, invalidArgumentsError("--clip cannot be combined with --reveal")
	}
	cmd.itemPath = path
	cmd.sessionID = c.String("session")
	cmd.clipboard = c.Bool("clip")
	cmd.reveal = c.Bool("reveal")
	cmd.json = c.Bool("json")
	cmd.field = ""
	if len(selectors) == 1 {
		cmd.field = selectors[0]
	}
	return ctx, nil
}

// Run displays the selected item from its vault.
func (cmd *ShowCmd) Run(ctx context.Context, _ *urfave.Command) error {
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
		value, err := itemFieldAt(item, cmd.itemType, cmd.field, cmd.currentTime())
		if err != nil {
			return err
		}
		if cmd.clipboard {
			if err := cmd.copyValue(ctx, value); err != nil {
				return err
			}
			if err := cmd.Infof("%s copied to clipboard\n", cmd.field); err != nil {
				return err
			}
			return nil
		}
		if cmd.json {
			encoded, err := json.Marshal(value)
			if err != nil {
				return fmt.Errorf("could not encode field as JSON: %w", err)
			}
			err = cmd.Printf("%s\n", string(encoded))
			return err
		}
		err = cmd.Printf("%s", value)
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
		if err := cmd.Infof("%s copied to clipboard\n", strings.ToUpper(label[:1])+label[1:]); err != nil {
			return err
		}
		return nil
	}
	if cmd.json {
		encoded, err := json.Marshal(showJSON{Item: item, itemType: cmd.itemType})
		if err != nil {
			return fmt.Errorf("could not encode item as JSON: %w", err)
		}
		return cmd.Printf("%s\n", string(encoded))
	}
	if err := cmd.Printf("ID: %s\n", item.ID); err != nil {
		return err
	}
	switch cmd.itemType {
	case model.LoginItemType:
		if item.Scope != nil {
			for _, url := range item.Scope.URLs {
				if err := cmd.Printf("URL: %s\n", safeTerminalText(url)); err != nil {

					return err

				}
			}
		}
		credential, _, err := item.BasicAuth()
		if err != nil {
			return err
		}
		if err := cmd.Printf("Username: %s\n", safeTerminalText(usernameValue(credential))); err != nil {

			return err

		}
		if err := cmd.Printf("Password: %s\n", safeTerminalText(secretValue(passwordValue(credential), cmd.reveal))); err != nil {

			return err

		}
		if note, _, err := item.Note(); err == nil && note.Content.Value != "" {
			if err := cmd.Printf("Note: %s\n", safeTerminalText(note.Content.Value)); err != nil {

				return err

			}
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
				if err := cmd.Printf("TOTP: configured\n"); err != nil {

					return err

				}
				if cmd.reveal {
					if err := cmd.Printf("TOTP secret: %s\n", safeTerminalText(totp.Secret)); err != nil {

						return err

					}
				}
				if err := cmd.Printf("TOTP algorithm: %s\n", safeTerminalText(totp.Algorithm)); err != nil {

					return err

				}
				if err := cmd.Printf("TOTP digits: %d\n", totp.Digits); err != nil {

					return err

				}
				if err := cmd.Printf("TOTP period: %d seconds\n", totp.Period); err != nil {

					return err

				}
				if totp.Username != "" {
					if err := cmd.Printf("TOTP username: %s\n", safeTerminalText(totp.Username)); err != nil {

						return err

					}
				}
				if totp.Issuer != "" {
					if err := cmd.Printf("TOTP issuer: %s\n", safeTerminalText(totp.Issuer)); err != nil {

						return err

					}
				}
			}
		}
	case model.PasswordItemType:
		credential, _, err := item.BasicAuth()
		if err != nil {
			return err
		}
		if err := cmd.Printf("Password: %s\n", safeTerminalText(secretValue(passwordValue(credential), cmd.reveal))); err != nil {

			return err

		}
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
		if err := cmd.Printf("Private key: %s\n", safeTerminalText(secretValue(privateKey, cmd.reveal))); err != nil {

			return err

		}
		if err := cmd.Printf("Public key: %s", safeTerminalText(publicKey)); err != nil {

			return err

		}
		if err := cmd.Printf("Fingerprint: %s\n", safeTerminalText(fingerprint)); err != nil {

			return err

		}
		if credential.CreationDate != nil {
			if err := cmd.Printf("Key creation date: %s\n", safeTerminalText(editableDisplayValue(credential.CreationDate, cmd.reveal))); err != nil {

				return err

			}
		}
		if credential.ExpiryDate != nil {
			if err := cmd.Printf("Key expiry date: %s\n", safeTerminalText(editableDisplayValue(credential.ExpiryDate, cmd.reveal))); err != nil {

				return err

			}
		}
		if credential.KeyGenerationSource != nil {
			if err := cmd.Printf("Key generation source: %s\n", safeTerminalText(editableDisplayValue(credential.KeyGenerationSource, cmd.reveal))); err != nil {

				return err

			}
		}
		if credential, _, err := item.BasicAuth(); err == nil && credential.Password != nil && credential.Password.Value != "" {
			if err := cmd.Printf("Passphrase: %s\n", safeTerminalText(secretValue(credential.Password.Value, cmd.reveal))); err != nil {

				return err

			}
		}
		addToAgent := "No"
		if item.Metadata.AddToAgent {
			addToAgent = "Yes"
		}
		if err := cmd.Printf("Add to agent: %s\n", addToAgent); err != nil {

			return err

		}
	case model.NoteItemType:
		note, _, err := item.Note()
		if err != nil {
			return err
		}
		if err := cmd.Printf("Note: %s\n", safeTerminalText(note.Content.Value)); err != nil {

			return err

		}
	}
	groups, err := item.CustomFields()
	if err != nil {
		return err
	}
	if len(groups) > 0 {
		if err := cmd.Printf("\nCustom fields:\n"); err != nil {

			return err

		}
		for _, group := range groups {
			if group.Credential.Label != "" {
				if err := cmd.Printf("  [%s]\n", safeTerminalText(group.Credential.Label)); err != nil {

					return err

				}
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
				if err := cmd.Printf("  %s: %s\n", safeTerminalText(label), safeTerminalText(value)); err != nil {

					return err

				}
			}
		}
	}
	if len(item.Tags) > 0 {
		if err := cmd.Printf("Tags: %s\n", safeTerminalText(strings.Join(item.Tags, ", "))); err != nil {

			return err

		}
	}

	if item.ModifiedAt != nil {
		if err := cmd.Printf("Modified: %s\n", time.Unix(int64(*item.ModifiedAt), 0).Local().Format(time.RFC1123)); err != nil {

			return err

		}
	}
	if item.CreationAt != nil {
		if err := cmd.Printf("Created: %s\n", time.Unix(int64(*item.CreationAt), 0).Local().Format(time.RFC1123)); err != nil {

			return err

		}
	}
	files, err := item.Files()
	if err != nil {
		return fmt.Errorf("cannot read attachment metadata: %w", err)
	}
	if len(files) > 0 {
		if err := cmd.Printf("\nAttachments: %d\n", len(files)); err != nil {

			return err

		}
	}
	history, err := item.History()
	if err != nil {
		return fmt.Errorf("cannot read item history: %w", err)
	}
	if len(history.Entries) > 0 {
		if len(files) == 0 {
			if err := cmd.Printf("\n"); err != nil {

				return err

			}
		}
		if err := cmd.Printf("History: %d versions\n", len(history.Entries)); err != nil {

			return err

		}
	}

	return nil
}

// secretValue masks a secret unless reveal is enabled.
func secretValue(value string, reveal bool) string {
	if reveal {
		return value
	}
	return "********"
}

// editableDisplayValue formats a custom field while masking hidden values.
func editableDisplayValue(field *cxf.EditableField, reveal bool) string {
	if field == nil {
		return ""
	}
	if field.FieldType == cxf.FieldTypeConcealedString && !reveal {
		return "********"
	}
	return field.Value
}

// safeTerminalText escapes terminal control characters in untrusted text.
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

// copyValue writes a value to the clipboard and schedules its removal.
func (cmd *ShowCmd) copyValue(ctx context.Context, value string) error {
	ctx, cancel := context.WithTimeout(ctx, clipboardWriteTimeout)
	defer cancel()
	return cmd.command.clipboard(ctx, []byte(value))
}

// itemDefaultSecret returns the default secret and its display label.
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

// itemField resolves a named field using the current time for generated values.
func itemField(item *model.Item, itemType model.ItemType, name string) (string, error) {
	return itemFieldAt(item, itemType, name, time.Now())
}

// itemFieldAt resolves a named field at the supplied time.
func itemFieldAt(item *model.Item, itemType model.ItemType, name string, now time.Time) (string, error) {
	var values []string
	add := func(label, value string) {
		if canonicalFieldName(label) == canonicalFieldName(name) {
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
		if itemType == model.LoginItemType {
			note, index, err := item.Note()
			if err != nil {
				return "", err
			}
			if index >= 0 {
				add("note", note.Content.Value)
			}
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
		add("totp-secret", totp.Secret)
		if canonicalFieldName(name) == "totp" {
			code, err := generateTOTPCode(totp, now)
			if err != nil {
				return "", fmt.Errorf("could not generate TOTP code: %w", err)
			}
			add("totp", code)
		}
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

// canonicalFieldName normalizes aliases for standard item fields.
func canonicalFieldName(name string) string {
	name = strings.ToLower(name)
	switch name {
	case "totp secret", "totp-secret":
		return "totp-secret"
	default:
		return name
	}
}

// currentTime returns the command's configured clock time.
func (cmd *ShowCmd) currentTime() time.Time {
	if cmd.now != nil {
		return cmd.now()
	}
	return time.Now()
}

// generateTOTPCode returns the current code for an authenticator credential.
func generateTOTPCode(credential cxf.TOTP, now time.Time) (string, error) {
	if credential.Secret == "" {
		return "", fmt.Errorf("secret is empty")
	}
	if credential.Period == 0 {
		return "", fmt.Errorf("period must be greater than zero")
	}
	if credential.Digits == 0 || credential.Digits > 10 {
		return "", fmt.Errorf("digit count must be between 1 and 10")
	}
	algorithm := strings.ToUpper(credential.Algorithm)
	switch algorithm {
	case "SHA1", "SHA256", "SHA512":
	default:
		return "", fmt.Errorf("unsupported algorithm %q", credential.Algorithm)
	}
	code, err := otp.TOTPFromBase32(paw.TOTPHash(algorithm).Hasher(), credential.Secret, now.UTC(), int(credential.Period), int(credential.Digits))
	if err != nil {
		return "", err
	}
	return code, nil
}
