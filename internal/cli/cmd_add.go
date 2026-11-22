// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	"charm.land/huh/v2"
	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

// AddCmd implements the command that adds an item to a vault.
type AddCmd struct {
	*command
	sessionID string
	itemPath
	vaultKey       *paw.Key
	importPath     string
	tags           []string
	username       string
	loginURL       string
	usernameSet    bool
	urlSet         bool
	randomPassword bool
	passwordStdin  bool
	totpStdin      bool
	addToAgent     bool
	addToAgentSet  bool
	wizard         bool
	input          io.Reader
}

// newAddCommand constructs the item-add command.
func newAddCommand(base *command) *urfave.Command {
	cmd := &AddCmd{command: base, wizard: true}
	return &urfave.Command{
		Name:   "add",
		Usage:  "Adds an item to a vault",
		Before: cmd.Validate,
		Action: cmd.Run,
		Commands: []*urfave.Command{
			newAddLoginCommand(base),
			newAddPasswordCommand(base),
			newAddNoteCommand(base),
			newAddSSHKeyCommand(base),
		},
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.StringSliceFlag{
				Name:  "tag",
				Usage: "Associate a tag with the new item; may be repeated",
			},
		},
		OnUsageError: onUsageError,
	}
}

// Validate checks the main add command arguments and flags.
func (cmd *AddCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if c.Args().Present() {
		if c.Command(c.Args().First()) != nil {
			return ctx, nil
		}
		return ctx, commandGroupAction(ctx, c)
	}
	return ctx, cmd.validateCommonFlags(c)
}

// validateItem reads the vault, item name and flags common to the subcommands.
func (cmd *AddCmd) validateItem(c *urfave.Command) error {
	if err := validateArgs(c, 2, 2); err != nil {
		return err
	}
	cmd.vaultName = c.Args().Get(0)
	cmd.itemName = c.Args().Get(1)
	if strings.TrimSpace(cmd.vaultName) == "" || strings.TrimSpace(cmd.itemName) == "" {
		return invalidArgumentsError("vault and item name cannot be empty")
	}
	if strings.Contains(cmd.itemName, "/") {
		return invalidArgumentsError("item name cannot contain /")
	}
	return cmd.validateCommonFlags(c)
}

// validateCommonFlags reads the session and validates the supplied tags.
func (cmd *AddCmd) validateCommonFlags(c *urfave.Command) error {
	cmd.sessionID = c.String("session")
	cmd.tags = nil
	for _, tag := range c.StringSlice("tag") {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			return invalidArgumentsError("--tag cannot be empty")
		}
		cmd.tags = append(cmd.tags, tag)
	}
	return nil
}

// Run creates and stores a CXF item in the selected vault.
func (cmd *AddCmd) Run(ctx context.Context, _ *urfave.Command) error {
	if cmd.wizard {
		if err := cmd.selectItem(); err != nil {
			return err
		}
	}
	s := cmd.storage
	appState, err := s.LoadAppState()
	if err != nil {
		return err
	}
	key := cmd.vaultKey
	if key == nil {
		key, err = cmd.addVaultKey()
		if err != nil {
			return err
		}
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
		item, err = cmd.newNote()
	case model.PasswordItemType:
		item, err = cmd.newPassword()
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
	if cmd.wizard {
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
	return cmd.Printf("%s\n", item.ID)
}

// selectItem asks for the vault, type and name in the interactive add command.
func (cmd *AddCmd) selectItem() error {
	if err := cmd.prompter.checkTerminal(); err != nil {
		return err
	}
	vaults, err := cmd.storage.Vaults()
	if err != nil {
		return err
	}
	if len(vaults) == 0 {
		return fmt.Errorf("no vaults available")
	}
	options := make([]huh.Option[string], 0, len(vaults))
	for _, vault := range vaults {
		label := fmt.Sprintf("%s (%s)", safeTerminalText(vault.Name), shortVaultID(vault.ID))
		options = append(options, huh.NewOption(label, storage.FormatVaultID(vault.ID)))
	}
	if err := cmd.prompter.runHuh(huh.NewSelect[string]().Title("Vault").Options(options...).Value(&cmd.vaultName)); err != nil {
		return err
	}
	key, err := cmd.addVaultKey()
	if err != nil {
		return err
	}
	cmd.vaultKey = key
	var types []huh.Option[model.ItemType]
	for _, kind := range []model.ItemType{model.LoginItemType, model.PasswordItemType, model.NoteItemType, model.SSHKeyItemType} {
		types = append(types, huh.NewOption(kind.Label(), kind))
	}
	if err := cmd.prompter.runHuh(huh.NewSelect[model.ItemType]().Title("Item type").Options(types...).Value(&cmd.itemType)); err != nil {
		return err
	}
	name, err := cmd.prompter.ask("Item name")
	if err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" || strings.Contains(name, "/") {
		return fmt.Errorf("item name must be nonempty and cannot contain /")
	}
	cmd.itemName = name
	return nil
}

// addVaultKey opens the selected vault without prompting when stdin is reserved.
func (cmd *AddCmd) addVaultKey() (*paw.Key, error) {
	if cmd.sessionID == "" {
		if cmd.input != nil {
			return nil, fmt.Errorf("an active vault session is required when reading from stdin")
		}
		if err := cmd.prompter.checkTerminal(); err != nil {
			return nil, err
		}
		return cmd.loadVaultKey(cmd.vaultName, "")
	}
	info, err := cmd.resolveVaultInfo(cmd.vaultName)
	if err != nil {
		return nil, err
	}
	cmd.vaultID = info.ID
	key, err := cmd.loadVaultKeyWithSession(info.ID, cmd.sessionID)
	if err != nil {
		return nil, fmt.Errorf("could not use vault session: %w", err)
	}
	return key, nil
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
				if _, err := fmt.Fprintln(cmd.errOut, "Section name cannot be empty"); err != nil {
					return err
				}
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
			if _, err := fmt.Fprintln(cmd.errOut, "Choose a number from 1 to 4"); err != nil {
				return err
			}
		case "":
			return nil
		default:
			if _, err := fmt.Fprintln(cmd.errOut, "Choose a number from 1 to 5"); err != nil {
				return err
			}
		}
	}
}

// manageNewItemTags prompts for tags to associate with a new item.
func (cmd *AddCmd) manageNewItemTags(item *model.Item, vault *model.Vault) error {
	tags := vaultTagNames(vault)
	if len(tags) > 0 {
		if _, err := fmt.Fprintf(cmd.errOut, "Existing tags: %s\n", safeTerminalText(strings.Join(tags, ", "))); err != nil {
			return err
		}
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
