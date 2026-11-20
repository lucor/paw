// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"crypto"
	"fmt"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// AddCmd implements the command that adds an item to a vault.
type AddCmd struct {
	*command
	sessionID string
	itemPath
	importPath string
	tags       []string
}

// newAddCommand constructs the item-add command.
func newAddCommand(base *command) *urfave.Command {
	cmd := &AddCmd{command: base}
	c := &urfave.Command{
		Name:        "add",
		Aliases:     []string{"insert"},
		Usage:       "Adds an item to the vault (alias: insert)",
		ArgsUsage:   "VAULT/ITEM_TYPE/ITEM_NAME",
		Description: vaultReferenceHelp,
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.StringSliceFlag{
				Name:  "tag",
				Usage: "Associate a tag with the new item; may be repeated",
			},
			&urfave.StringFlag{
				Name:    "input",
				Aliases: []string{"i"},
				Usage:   "Import an SSH key from FILE",
			},
		},
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the item path and add flags.
func (cmd *AddCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return ctx, err
	}
	tags := c.StringSlice("tag")
	for index, tag := range tags {
		tags[index] = strings.TrimSpace(tag)
		if tags[index] == "" {
			return ctx, fmt.Errorf("--tag cannot be empty")
		}
	}
	cmd.itemPath = path
	cmd.sessionID = c.String("session")
	cmd.importPath = c.String("input")
	cmd.tags = tags
	return ctx, nil
}

// Run creates and stores a CXF item in the selected vault.
func (cmd *AddCmd) Run(ctx context.Context, _ *urfave.Command) error {
	s := cmd.storage
	appState, err := s.LoadAppState()
	if err != nil {
		return err
	}
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return err
	}
	vault, err := cmd.loadVault(key)
	if err != nil {
		return err
	}
	items, err := vault.FilterItems(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.Title == cmd.itemName {
			return fmt.Errorf("item with same name already exists")
		}
	}
	var item *model.Item
	switch cmd.itemType {
	case model.NoteItemType:
		var content string
		if cmd.prompter.interactive {
			content, err = cmd.prompter.editMultiline("Note", "")
		} else {
			content, err = cmd.prompter.ask("Note")
		}
		if err != nil {
			return err
		}
		created, createErr := model.NewNote(cmd.itemName, content)
		if createErr != nil {
			return createErr
		}
		item = created
	case model.PasswordItemType:
		password, err := cmd.generatePassword(paw.CustomPassword)
		if err != nil {
			return err
		}
		created, createErr := model.NewPassword(cmd.itemName, password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format})
		if createErr != nil {
			return createErr
		}
		item = created
	case model.LoginItemType:
		item, err = cmd.newLogin()
	case model.SSHKeyItemType:
		item, err = cmd.newSSHKey()
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}
	if err != nil {
		return err
	}
	for _, tag := range cmd.tags {
		if !slices.Contains(item.Tags, tag) {
			item.Tags = append(item.Tags, tag)
		}
	}
	if cmd.prompter.interactive {
		if err := cmd.addOptionalData(item, vault); err != nil {
			return err
		}
	}
	if err := s.StoreItem(vault, item); err != nil {
		return err
	}
	appState.Modified = cmd.now().UTC()
	if err := s.StoreAppState(appState); err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] item %q added\n", safeTerminalText(cmd.itemName))
	return nil
}

// addOptionalData prompts for custom fields, tags, and authenticator data.
func (cmd *AddCmd) addOptionalData(item *model.Item, vault *model.Vault) error {
	add, err := cmd.prompter.askYesNo("Add custom fields or tags?", false)
	if err != nil || !add {
		return err
	}
	for {
		menu := "1. Add custom field  2. Add section  3. Manage tags  4. Done"
		if cmd.itemType == model.LoginItemType {
			menu = "1. Add custom field  2. Add section  3. Manage tags  4. Add authenticator  5. Done"
		}
		choice, err := cmd.prompter.ask(menu)
		if err != nil {
			return err
		}
		switch choice {
		case "1":
			field, err := cmd.readCustomField(false, cxf.EditableField{})
			if err != nil {
				return err
			}
			if strings.TrimSpace(field.Label) == "" {
				return fmt.Errorf("field name cannot be empty")
			}
			section, err := cmd.prompter.ask("Section (leave empty for none)")
			if err != nil {
				return err
			}
			if err := appendField(item, section, field); err != nil {
				return err
			}
		case "2":
			name, err := cmd.prompter.ask("Section name")
			if err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				fmt.Fprintln(cmd.errOut, "Section name cannot be empty")
				continue
			}
			if err := item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: name, Fields: []cxf.EditableField{}}); err != nil {
				return err
			}
		case "3":
			if err := cmd.manageNewItemTags(item, vault); err != nil {
				return err
			}
		case "4":
			if cmd.itemType != model.LoginItemType {
				return nil
			}
			if err := cmd.addTOTP(item); err != nil {
				return err
			}
		case "5":
			if cmd.itemType == model.LoginItemType {
				return nil
			}
			fmt.Fprintln(cmd.errOut, "Choose a number from 1 to 4")
		case "":
			return nil
		default:
			fmt.Fprintln(cmd.errOut, "Choose a number from 1 to 5")
		}
	}
}

// addTOTP prompts for and attaches an authenticator credential.
func (cmd *AddCmd) addTOTP(item *model.Item) error {
	credentials, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return err
	}
	if credentials.TOTP != nil {
		return fmt.Errorf("item already has an authenticator")
	}
	secret, err := cmd.prompter.askPassword("Authenticator secret")
	if err != nil {
		return err
	}
	if secret == "" {
		return nil
	}
	credential := cxf.TOTP{
		Type:      cxf.CredentialTypeTOTP,
		Secret:    normalizeTOTPSecret(secret),
		Algorithm: strings.ToLower(string(paw.TOTPHashDefault)),
		Period:    uint16(paw.TOTPIntervalDefault),
		Digits:    uint16(paw.TOTPDigitsDefault),
	}
	if err := configureTOTP(cmd.prompter, &credential); err != nil {
		return err
	}
	if err := credential.Validate(); err != nil {
		return fmt.Errorf("invalid authenticator secret: %w", err)
	}
	return item.SetTOTP(-1, credential)
}

// manageNewItemTags prompts for tags to associate with a new item.
func (cmd *AddCmd) manageNewItemTags(item *model.Item, vault *model.Vault) error {
	tags := vaultTagNames(vault)
	if len(tags) > 0 {
		fmt.Fprintf(cmd.errOut, "Existing tags: %s\n", safeTerminalText(strings.Join(tags, ", ")))
	}
	for {
		value, err := cmd.prompter.ask("Tag to associate (empty to finish)")
		if err != nil || value == "" {
			return err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if !slices.Contains(item.Tags, value) {
			item.Tags = append(item.Tags, value)
		}
	}
}

// vaultTagNames returns the sorted, unique tags used in a vault.
func vaultTagNames(vault *model.Vault) []string {
	seen := make(map[string]bool)
	var tags []string
	if len(vault.Accounts) == 0 {
		return tags
	}
	for _, item := range vault.Accounts[0].Items {
		for _, tag := range item.Tags {
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	sort.Strings(tags)
	return tags
}

// readCustomField prompts for a custom field and its value.
func (cmd *AddCmd) readCustomField(editing bool, old cxf.EditableField) (cxf.EditableField, error) {
	field := old
	if editing {
		update, err := cmd.prompter.askYesNo("Replace field value?", false)
		if err != nil || !update {
			return field, err
		}
		kind := customFieldType(field)
		if strings.Contains(kind, "multiline") {
			field.Value, err = cmd.readMultiline(kind == "hidden-multiline")
			return field, err
		}
		var value string
		if kind == "hidden" {
			value, err = cmd.prompter.askPassword("Value (empty keeps the current value; :clear clears it)")
		} else {
			value, err = cmd.prompter.ask("Value (empty keeps the current value; :clear clears it)")
		}
		if value == ":clear" {
			field.Value = ""
		} else if value != "" {
			field.Value = value
		}
		return field, err
	}
	labelPrompt := "Field name"
	if editing {
		labelPrompt = "Field name [" + safeTerminalText(old.Label) + "]"
	}
	label, err := cmd.prompter.ask(labelPrompt)
	if err != nil {
		return field, err
	}
	if label != "" {
		field.Label = label
	}
	kind, err := cmd.prompter.ask("Type (text, hidden, multiline, hidden-multiline)")
	if err != nil {
		return field, err
	}
	if kind == "" && editing {
		kind = customFieldType(field)
	}
	if kind == "" {
		kind = "text"
	}
	if !validCustomFieldType(kind) {
		return field, fmt.Errorf("unsupported custom field type %q", kind)
	}
	field.FieldType = cxf.FieldTypeString
	if kind == "hidden" || kind == "hidden-multiline" {
		field.FieldType = cxf.FieldTypeConcealedString
	}
	if strings.Contains(kind, "multiline") {
		field.Value, err = cmd.readMultiline(kind == "hidden-multiline")
	} else if kind == "hidden" {
		value, e := cmd.prompter.askPassword("Value (empty keeps the current value)")
		err = e
		if value == ":clear" {
			field.Value = ""
		} else if value != "" {
			field.Value = value
		}
	} else {
		value, e := cmd.prompter.ask("Value (empty keeps the current value)")
		err = e
		if value == ":clear" {
			field.Value = ""
		} else if value != "" {
			field.Value = value
		}
	}
	return field, err
}

// readMultiline reads lines until a period-only line is entered.
func (cmd *AddCmd) readMultiline(hidden bool) (string, error) {
	var lines []string
	for {
		var line string
		var err error
		if hidden {
			line, err = cmd.prompter.askPassword("Value line (enter . on a line to finish)")
		} else {
			line, err = cmd.prompter.ask("Value line (enter . on a line to finish)")
		}
		if err != nil {
			return "", err
		}
		if line == "." {
			return strings.Join(lines, "\n"), nil
		}
		lines = append(lines, line)
	}
}

// appendField adds a custom field to the requested section.
func appendField(item *model.Item, section string, field cxf.EditableField) error {
	groups, err := item.CustomFields()
	if err != nil {
		return err
	}
	var destination *model.CustomFieldsGroup
	for _, group := range groups {
		if group.Credential.Label != section {
			continue
		}
		if destination != nil && section != "" {
			return fmt.Errorf("section %q is ambiguous", section)
		}
		if destination == nil {
			destination = &group
		}
	}
	if destination != nil {
		original := make([]int, len(destination.Credential.Fields)+1)
		for i := range destination.Credential.Fields {
			original[i] = i
		}
		original[len(original)-1] = -1
		destination.Credential.Fields = append(destination.Credential.Fields, field)
		return item.ReplaceCustomFields(destination.Index, destination.Credential, original)
	}
	return item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: section, Fields: []cxf.EditableField{field}})
}

// customFieldType returns the display type for a custom field.
func customFieldType(field cxf.EditableField) string {
	long := strings.Contains(field.Value, "\n") || utf8.RuneCountInString(field.Value) > 160
	if field.FieldType == cxf.FieldTypeConcealedString {
		if long {
			return "hidden-multiline"
		}
		return "hidden"
	}
	if long {
		return "multiline"
	}
	return "text"
}

// newLogin prompts for login fields and builds a CXF item.
func (cmd *AddCmd) newLogin() (*model.Item, error) {
	url, err := cmd.prompter.ask("URL")
	if err != nil {
		return nil, err
	}
	url, err = normalizeLoginURL(url)
	if err != nil {
		return nil, err
	}
	username, err := cmd.prompter.ask("Username")
	if err != nil {
		return nil, err
	}
	password, err := cmd.generatePassword(paw.CustomPassword)
	if err != nil {
		return nil, err
	}
	item, err := model.NewLogin(cmd.itemName, username, password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format})
	if err != nil {
		return nil, err
	}
	item.Scope = &cxf.CredentialScope{URLs: []string{url}, AndroidApps: []cxf.AndroidAppID{}}
	var note string
	if cmd.prompter.interactive {
		addNote, err := cmd.prompter.askYesNo("Add a note?", false)
		if err != nil {
			return nil, err
		}
		if addNote {
			note, err = cmd.prompter.editMultiline("Note", "")
			if err != nil {
				return nil, err
			}
		}
	} else {
		note, err = cmd.prompter.ask("Note")
	}
	if err != nil {
		return nil, err
	}
	if note != "" {
		credential := cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: note}}
		if err := item.SetNote(-1, credential); err != nil {
			return nil, err
		}
	}
	return item, nil
}

// normalizeLoginURL adds HTTPS to a host-only URL and validates its scheme.
func normalizeLoginURL(value string) (string, error) {
	loginURL := paw.NewLoginURL()
	if err := loginURL.Set(value); err != nil {
		return "", fmt.Errorf("enter a valid website address")
	}
	if loginURL.String() != "" && loginURL.URL().Scheme != "https" {
		return "", fmt.Errorf("website must use HTTPS")
	}
	return loginURL.String(), nil
}

// newSSHKey imports or generates an SSH key and builds a CXF item.
func (cmd *AddCmd) newSSHKey() (*model.Item, error) {
	var privateKey crypto.PrivateKey
	var publicKey ssh.PublicKey
	var passphrase string
	if cmd.importPath != "" {
		data, err := cmd.readFile(cmd.importPath)
		if err != nil {
			return nil, err
		}
		parsed, err := sshkey.ParseKey(data)
		if err != nil {
			if _, ok := err.(*ssh.PassphraseMissingError); !ok {
				return nil, err
			}
			passphrase, err = cmd.prompter.askPassword("Private Key is password protected")
			if err != nil {
				return nil, err
			}
			parsed, err = sshkey.ParseKeyWithPassphrase(data, []byte(passphrase))
			if err != nil {
				return nil, err
			}
		}
		privateKey, publicKey = parsed.PrivateKey(), parsed.PublicKey()
	} else {
		generated, err := cmd.generateSSHKey()
		if err != nil {
			return nil, err
		}
		parsed, err := sshkey.ParseKey([]byte(generated.PrivateKey))
		if err != nil {
			return nil, err
		}
		privateKey, publicKey = parsed.PrivateKey(), parsed.PublicKey()
	}
	fmt.Fprintln(cmd.out, "The key fingerprint is:")
	fmt.Fprintln(cmd.out, ssh.FingerprintSHA256(publicKey))
	if cmd.importPath != "" {
		ok, err := cmd.prompter.askYesNo("Continue?", true)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, errCanceled
		}
	}
	addToAgent, err := cmd.prompter.askYesNo("Add to SSH Agent?", false)
	if err != nil {
		return nil, err
	}
	credential, err := model.EncodeSSHKey(privateKey, publicKey.Type(), cmd.itemName)
	if err != nil {
		return nil, fmt.Errorf("cannot encode SSH private key")
	}
	item, err := model.NewSSHKey(cmd.itemName, credential, passphrase, model.PasswordSettings{Mode: paw.CustomPassword})
	if err != nil {
		return nil, err
	}
	item.Metadata.AddToAgent = addToAgent
	return item, nil
}

// generatePassword prompts for a password using the supported generators.
func (cmd *AddCmd) generatePassword(defaultMode paw.PasswordMode) (*paw.PasswordSpec, error) {
	return (&PwGenCmd{command: cmd.command}).Pwgen([]paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}, defaultMode)
}
