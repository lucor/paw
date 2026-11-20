// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"crypto"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// EditCmd implements the command that edits an item in a vault.
type EditCmd struct {
	*command
	sessionID string
	itemPath
	field       string
	typeName    string
	remove      bool
	tag         string
	name        string
	importPath  string
	generateKey bool
	input       io.Reader
}

// newEditCommand constructs the item-edit command.
func newEditCommand(base *command) *urfave.Command {
	cmd := &EditCmd{command: base}
	c := &urfave.Command{
		Name:        "edit",
		Usage:       "Edits an item; --field reads a custom field value from stdin",
		ArgsUsage:   "VAULT/ITEM_TYPE/ITEM_NAME",
		Description: vaultReferenceHelp + " Use --name to rename an item, --tag to associate one tag, --input to import or --generate-key to replace an SSH key; --remove removes a field or tag association.",
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.StringFlag{
				Name:  "field",
				Usage: "Set or remove a custom field using Field or Section/Field",
			},
			&urfave.StringFlag{
				Name:  "type",
				Usage: "Custom field UI type: text, hidden, multiline, hidden-multiline",
			},
			&urfave.BoolFlag{
				Name:  "remove",
				Usage: "Remove the selected custom field or tag association",
			},
			&urfave.StringFlag{
				Name:  "tag",
				Usage: "Associate a vault tag with this item",
			},
			&urfave.StringFlag{
				Name:  "name",
				Usage: "Set the item title",
			},
			&urfave.StringFlag{
				Name:    "input",
				Aliases: []string{"i"},
				Usage:   "Import a replacement SSH key from FILE",
			},
			&urfave.BoolFlag{
				Name:  "generate-key",
				Usage: "Generate a replacement SSH key",
			},
		},
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the item path and edit flags.
func (cmd *EditCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return ctx, err
	}
	field, typeName, tag, name, remove := c.String("field"), c.String("type"), strings.TrimSpace(c.String("tag")), c.String("name"), c.Bool("remove")
	if c.IsSet("field") && field == "" {
		return ctx, fmt.Errorf("--field requires a field name")
	}
	if c.IsSet("tag") && tag == "" {
		return ctx, fmt.Errorf("--tag requires a tag name")
	}
	if typeName != "" && field == "" {
		return ctx, fmt.Errorf("--type requires --field")
	}
	if typeName != "" && !validCustomFieldType(typeName) {
		return ctx, fmt.Errorf("unsupported custom field type %q", typeName)
	}
	if remove && typeName != "" {
		return ctx, fmt.Errorf("--type cannot be combined with --remove")
	}
	if remove && field == "" && tag == "" {
		return ctx, fmt.Errorf("--remove requires --field or --tag")
	}
	if field != "" && tag != "" && remove {
		return ctx, fmt.Errorf("--remove cannot remove a field and tag in the same operation")
	}
	if c.IsSet("name") && strings.TrimSpace(name) == "" {
		return ctx, fmt.Errorf("--name cannot be empty")
	}
	if name != "" && remove {
		return ctx, fmt.Errorf("--name cannot be combined with --remove")
	}
	importPath := c.String("input")
	if c.IsSet("input") && importPath == "" {
		return ctx, fmt.Errorf("--input requires a file path")
	}
	if importPath != "" && path.itemType != model.SSHKeyItemType {
		return ctx, fmt.Errorf("--input is only supported for SSH key items")
	}
	if importPath != "" && remove {
		return ctx, fmt.Errorf("--input cannot be combined with --remove")
	}
	generateKey := c.Bool("generate-key")
	if generateKey && path.itemType != model.SSHKeyItemType {
		return ctx, fmt.Errorf("--generate-key is only supported for SSH key items")
	}
	if generateKey && importPath != "" {
		return ctx, fmt.Errorf("--generate-key cannot be combined with --input")
	}
	if generateKey && remove {
		return ctx, fmt.Errorf("--generate-key cannot be combined with --remove")
	}
	cmd.itemPath = path
	cmd.sessionID = c.String("session")
	cmd.field = field
	cmd.typeName = typeName
	cmd.tag = tag
	cmd.name = name
	cmd.importPath = importPath
	cmd.generateKey = generateKey
	cmd.remove = remove
	cmd.input = c.Reader
	return ctx, nil
}

// Run loads, updates, and stores a CXF item.
func (cmd *EditCmd) Run(ctx context.Context, _ *urfave.Command) error {
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
	summary, err := vault.FindItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}
	item, err := s.LoadItem(vault, summary.ID)
	if err != nil {
		return err
	}
	if cmd.name != "" {
		err = cmd.renameItem(item, vault, cmd.name)
	}
	if err == nil && (cmd.importPath != "" || cmd.generateKey) {
		err = cmd.replaceSSHKey(item)
	}
	if err == nil && (cmd.field != "" || cmd.tag != "") {
		err = cmd.editCustomAndTags(item)
	} else if err == nil && cmd.name == "" && cmd.importPath == "" && !cmd.generateKey && cmd.prompter.interactive {
		err = cmd.editInteractive(item, vault)
	} else if err == nil && cmd.name == "" && cmd.importPath == "" && !cmd.generateKey {
		switch cmd.itemType {
		case model.LoginItemType:
			err = cmd.editLogin(item)
		case model.PasswordItemType:
			err = cmd.editPassword(item)
		case model.NoteItemType:
			err = cmd.editNote(item)
		case model.SSHKeyItemType:
			err = cmd.editSSHKey(item)
		default:
			return fmt.Errorf("unsupported item type: %q", cmd.itemType)
		}
	}
	if err != nil {
		return err
	}
	now := uint64(cmd.now().UTC().Unix())
	previous, err := s.LoadItem(vault, item.ID)
	if err != nil {
		return err
	}
	updated, changed, err := model.PrepareItemEdit(previous, item, now, now)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintf(cmd.out, "[✓] item %q unchanged\n", safeTerminalText(item.Title))
		return nil
	}
	if err := s.StoreItem(vault, updated); err != nil {
		return err
	}
	appState.Modified = time.Unix(int64(now), 0).UTC()
	if err := s.StoreAppState(appState); err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] item %q modified\n", safeTerminalText(updated.Title))
	return nil
}

// renameItem updates the item title and records the change in its vault.
func (cmd *EditCmd) renameItem(item *model.Item, vault *model.Vault, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("item name cannot be empty")
	}
	if name == item.Title {
		return nil
	}
	items, err := vault.FilterItems("", cmd.itemType)
	if err != nil {
		return err
	}
	for _, existing := range items {
		if existing.ID != item.ID && existing.Title == name {
			return fmt.Errorf("item with same name already exists")
		}
	}
	item.Title = name
	return nil
}

// replaceSSHKey imports or generates a replacement SSH key.
func (cmd *EditCmd) replaceSSHKey(item *model.Item) error {
	var passphrase string
	var privateKey crypto.PrivateKey
	var publicKey ssh.PublicKey
	if cmd.generateKey {
		generated, err := cmd.generateSSHKey()
		if err != nil {
			return err
		}
		parsed, err := sshkey.ParseKey([]byte(generated.PrivateKey))
		if err != nil {
			return err
		}
		privateKey, publicKey = parsed.PrivateKey(), parsed.PublicKey()
	} else {
		data, err := cmd.readFile(cmd.importPath)
		if err != nil {
			return fmt.Errorf("cannot read SSH key file: %w", err)
		}
		parsed, err := sshkey.ParseKey(data)
		if err != nil {
			if _, ok := err.(*ssh.PassphraseMissingError); !ok {
				return err
			}
			passphrase, err = cmd.prompter.askPassword("Private key is password protected")
			if err != nil {
				return err
			}
			parsed, err = sshkey.ParseKeyWithPassphrase(data, []byte(passphrase))
			if err != nil {
				return err
			}
		}
		privateKey, publicKey = parsed.PrivateKey(), parsed.PublicKey()
	}
	credential, err := model.EncodeSSHKey(privateKey, publicKey.Type(), item.Title)
	if err != nil {
		return err
	}
	credentials, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return err
	}
	if err := item.SetSSHKey(credentials.SSHKeyIndex, credential); err != nil {
		return err
	}
	auth, authIndex, err := item.BasicAuth()
	if err != nil {
		return err
	}
	auth.Password, err = model.PasswordField(passphrase, model.PasswordSettingsFromField(auth.Password), auth.Password)
	if err != nil {
		return err
	}
	if err := item.SetBasicAuth(authIndex, auth); err != nil {
		return err
	}
	if cmd.prompter.interactive {
		addToAgent, err := cmd.prompter.askYesNo("Add to SSH agent", item.Metadata.AddToAgent)
		if err != nil {
			return err
		}
		item.Metadata.AddToAgent = addToAgent
	}
	fmt.Fprintf(cmd.errOut, "Replacement key fingerprint: %s\n", ssh.FingerprintSHA256(publicKey))
	return nil
}

// editInteractive prompts for changes to the item's standard fields.
func (cmd *EditCmd) editInteractive(item *model.Item, vault *model.Vault) error {
	for {
		fmt.Fprintf(cmd.errOut, "Editing: %s\n", safeTerminalText(item.Title))
		choice, err := cmd.prompter.ask("1. Standard fields  2. Custom fields  3. Tags  4. Save and exit")
		if err != nil {
			return err
		}
		var editErr error
		switch choice {
		case "1":
			name, err := cmd.prompter.askWithDefault("Name", item.Title)
			if err != nil {
				return err
			}
			if err := cmd.renameItem(item, vault, name); err != nil {
				return err
			}
			switch cmd.itemType {
			case model.LoginItemType:
				editErr = cmd.editLogin(item)
			case model.PasswordItemType:
				editErr = cmd.editPassword(item)
			case model.NoteItemType:
				editErr = cmd.editNote(item)
			case model.SSHKeyItemType:
				editErr = cmd.editSSHKey(item)
			default:
				editErr = fmt.Errorf("unsupported item type: %q", cmd.itemType)
			}
		case "2":
			editErr = cmd.editCustomInteractive(item)
		case "3":
			editErr = cmd.editTagsInteractive(item, vault)
		case "4", "":
			return nil
		default:
			fmt.Fprintln(cmd.errOut, "Choose 1, 2, 3, or 4")
		}
		if editErr != nil {
			return editErr
		}
	}
}

// editTagsInteractive prompts the user to toggle item tags.
func (cmd *EditCmd) editTagsInteractive(item *model.Item, vault *model.Vault) error {
	tags := vaultTagNames(vault)
	if len(tags) > 0 {
		fmt.Fprintf(cmd.errOut, "Vault tags: %s\n", safeTerminalText(strings.Join(tags, ", ")))
	}
	for {
		value, err := cmd.prompter.ask("Tag name to toggle (empty to finish)")
		if err != nil || value == "" {
			return err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if slices.Contains(item.Tags, value) {
			item.Tags = slices.DeleteFunc(item.Tags, func(tag string) bool { return tag == value })
		} else {
			item.Tags = append(item.Tags, value)
		}
	}
}

// editCustomInteractive prompts for custom field changes.
func (cmd *EditCmd) editCustomInteractive(item *model.Item) error {
	for {
		groups, err := item.CustomFields()
		if err != nil {
			return err
		}
		for gi, group := range groups {
			for fi, field := range group.Credential.Fields {
				label := safeTerminalText(field.Label)
				if group.Credential.Label != "" {
					label = safeTerminalText(group.Credential.Label) + "/" + label
				}
				fmt.Fprintf(cmd.errOut, "%d. %s (%s)\n", customFieldNumber(groups, gi, fi), label, customFieldType(field))
			}
		}
		choice, err := cmd.prompter.ask("1. Add field  2. Edit value  3. Rename field  4. Change UI type  5. Move field  6. Remove field  7. Manage sections  8. Done")
		if err != nil {
			return err
		}
		switch choice {
		case "1":
			field, err := (&AddCmd{command: cmd.command}).readCustomField(false, cxf.EditableField{})
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
		case "2", "3", "4", "5", "6":
			groupIndex, fieldIndex, ok, err := cmd.chooseCustomField(groups)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			group := groups[groupIndex]
			field := group.Credential.Fields[fieldIndex]
			switch choice {
			case "2":
				updated, err := (&AddCmd{command: cmd.command}).readCustomField(true, field)
				if err != nil {
					return err
				}
				group.Credential.Fields[fieldIndex] = updated
			case "3":
				label, err := cmd.prompter.ask("New field name")
				if err != nil {
					return err
				}
				if strings.TrimSpace(label) == "" {
					return fmt.Errorf("field name cannot be empty")
				}
				group.Credential.Fields[fieldIndex].Label = label
			case "4":
				kind, err := cmd.prompter.ask("Type (text, hidden, multiline, hidden-multiline)")
				if err != nil {
					return err
				}
				if !validCustomFieldType(kind) {
					return fmt.Errorf("unsupported custom field type %q", kind)
				}
				group.Credential.Fields[fieldIndex].FieldType = cxf.FieldTypeString
				if kind == "hidden" || kind == "hidden-multiline" {
					group.Credential.Fields[fieldIndex].FieldType = cxf.FieldTypeConcealedString
				}
			case "5":
				section, err := cmd.prompter.ask("Destination section (empty for no section)")
				if err != nil {
					return err
				}
				if section == group.Credential.Label {
					continue
				}
				if err := item.MoveCustomField(group.Index, fieldIndex, section); err != nil {
					return err
				}
				continue
			case "6":
				ok, err := cmd.prompter.askYesNo("Remove this field and its value?", false)
				if err != nil {
					return err
				}
				if !ok {
					continue
				}
				if err := removeCustomField(item, group, fieldIndex); err != nil {
					return err
				}
				continue
			}
			if choice != "5" && choice != "6" {
				if err := replaceCustomGroup(item, group); err != nil {
					return err
				}
			}
		case "7":
			if err := cmd.manageSections(item); err != nil {
				return err
			}
		case "8", "":
			return nil
		default:
			fmt.Fprintln(cmd.errOut, "Choose a number from 1 to 8")
		}
	}
}

// customFieldNumber returns a field's one-based position across all groups.
func customFieldNumber(groups []model.CustomFieldsGroup, groupIndex, fieldIndex int) int {
	number := fieldIndex + 1
	for index := 0; index < groupIndex; index++ {
		number += len(groups[index].Credential.Fields)
	}
	return number
}

// chooseCustomField prompts for a field and returns its group and field indexes.
func (cmd *EditCmd) chooseCustomField(groups []model.CustomFieldsGroup) (int, int, bool, error) {
	var count int
	for _, group := range groups {
		count += len(group.Credential.Fields)
	}
	if count == 0 {
		fmt.Fprintln(cmd.errOut, "No custom fields to select")
		return 0, 0, false, nil
	}
	value, err := cmd.prompter.ask("Field number")
	if err != nil {
		return 0, 0, false, err
	}
	number, err := strconv.Atoi(value)
	if err != nil || number < 1 || number > count {
		return 0, 0, false, fmt.Errorf("invalid custom field number")
	}
	for gi, group := range groups {
		if number <= len(group.Credential.Fields) {
			return gi, number - 1, true, nil
		}
		number -= len(group.Credential.Fields)
	}
	return 0, 0, false, nil
}

// replaceCustomGroup stores the updated fields for one custom field group.
func replaceCustomGroup(item *model.Item, group model.CustomFieldsGroup) error {
	original := make([]int, len(group.Credential.Fields))
	for index := range original {
		original[index] = index
	}
	return item.ReplaceCustomFields(group.Index, group.Credential, original)
}

// removeCustomField removes a field from its group while preserving other fields.
func removeCustomField(item *model.Item, group model.CustomFieldsGroup, fieldIndex int) error {
	group.Credential.Fields = append(group.Credential.Fields[:fieldIndex], group.Credential.Fields[fieldIndex+1:]...)
	original := make([]int, len(group.Credential.Fields))
	for index := range original {
		original[index] = index
		if index >= fieldIndex {
			original[index]++
		}
	}
	return item.ReplaceCustomFields(group.Index, group.Credential, original)
}

// manageSections prompts for custom field section changes.
func (cmd *EditCmd) manageSections(item *model.Item) error {
	groups, err := item.CustomFields()
	if err != nil {
		return err
	}
	var sections []model.CustomFieldsGroup
	for _, group := range groups {
		if group.Credential.Label != "" {
			sections = append(sections, group)
		}
	}
	choice, err := cmd.prompter.ask("1. Add section  2. Rename section  3. Remove section  4. Done")
	if err != nil {
		return err
	}
	switch choice {
	case "1":
		name, err := cmd.prompter.ask("Section name")
		if err != nil {
			return err
		}
		if strings.TrimSpace(name) == "" {
			return fmt.Errorf("section name cannot be empty")
		}
		return item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: name, Fields: []cxf.EditableField{}})
	case "2", "3":
		if len(sections) == 0 {
			fmt.Fprintln(cmd.errOut, "No sections to select")
			return nil
		}
		for index, section := range sections {
			fmt.Fprintf(cmd.errOut, "%d. %s (%d fields)\n", index+1, safeTerminalText(section.Credential.Label), len(section.Credential.Fields))
		}
		value, err := cmd.prompter.ask("Section number")
		if err != nil {
			return err
		}
		number, err := strconv.Atoi(value)
		if err != nil || number < 1 || number > len(sections) {
			return fmt.Errorf("invalid section number")
		}
		section := sections[number-1]
		if choice == "2" {
			name, err := cmd.prompter.ask("New section name")
			if err != nil {
				return err
			}
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("section name cannot be empty")
			}
			section.Credential.Label = name
			return replaceCustomGroup(item, section)
		}
		ok, err := cmd.prompter.askYesNo(fmt.Sprintf("Remove section %q and move its %d fields to the unsectioned fields?", safeTerminalText(section.Credential.Label), len(section.Credential.Fields)), false)
		if err != nil || !ok {
			return err
		}
		for range section.Credential.Fields {
			if err := item.MoveCustomField(section.Index, 0, ""); err != nil {
				return err
			}
		}
		return item.RemoveCustomFields(section.Index)
	case "4", "":
		return nil
	default:
		return fmt.Errorf("invalid section action")
	}
}

// editCustomAndTags applies the requested field or tag change from flags and stdin.
func (cmd *EditCmd) editCustomAndTags(item *model.Item) error {
	applyTag := func() {
		if cmd.tag == "" {
			return
		}
		if cmd.remove {
			item.Tags = slices.DeleteFunc(item.Tags, func(tag string) bool { return tag == cmd.tag })
		} else if !slices.Contains(item.Tags, cmd.tag) {
			item.Tags = append(item.Tags, cmd.tag)
		}
	}
	if cmd.field == "" {
		applyTag()
		return nil
	}
	section, label := splitFieldPath(cmd.field)
	groups, err := item.CustomFields()
	if err != nil {
		return err
	}
	groupIndex, fieldIndex := -1, -1
	for gi, group := range groups {
		if strings.Contains(cmd.field, "/") && group.Credential.Label != section {
			continue
		}
		for fi, field := range group.Credential.Fields {
			if strings.EqualFold(field.Label, label) {
				if fieldIndex >= 0 {
					return fmt.Errorf("field %q is ambiguous", cmd.field)
				}
				groupIndex, fieldIndex = gi, fi
			}
		}
	}
	if cmd.remove {
		if fieldIndex < 0 {
			return fmt.Errorf("field %q was not found", cmd.field)
		}
		group := groups[groupIndex]
		group.Credential.Fields = append(group.Credential.Fields[:fieldIndex], group.Credential.Fields[fieldIndex+1:]...)
		original := make([]int, len(group.Credential.Fields))
		for i := range original {
			original[i] = i
			if i >= fieldIndex {
				original[i]++
			}
		}
		err := item.ReplaceCustomFields(group.Index, group.Credential, original)
		if err == nil {
			applyTag()
		}
		return err
	}
	if cmd.typeName != "" && !validCustomFieldType(cmd.typeName) {
		return fmt.Errorf("unsupported custom field type %q", cmd.typeName)
	}
	if cmd.input == nil {
		return fmt.Errorf("custom field value stdin is unavailable")
	}
	value, err := io.ReadAll(cmd.input)
	if err != nil {
		return fmt.Errorf("cannot read custom field value: %w", err)
	}
	fieldType := cxf.FieldTypeString
	if cmd.typeName != "" {
		switch cmd.typeName {
		case "text", "multiline":
			fieldType = cxf.FieldTypeString
		case "hidden", "hidden-multiline":
			fieldType = cxf.FieldTypeConcealedString
		default:
			return fmt.Errorf("unsupported custom field type %q", cmd.typeName)
		}
	}
	if fieldIndex >= 0 {
		group := groups[groupIndex]
		field := &group.Credential.Fields[fieldIndex]
		field.Value = string(value)
		if cmd.typeName != "" {
			field.FieldType = fieldType
		}
		original := make([]int, len(group.Credential.Fields))
		for i := range original {
			original[i] = i
		}
		err := item.ReplaceCustomFields(group.Index, group.Credential, original)
		if err == nil {
			applyTag()
		}
		return err
	}
	if section == "" && strings.Contains(cmd.field, "/") {
		return fmt.Errorf("invalid custom field selector %q", cmd.field)
	}
	field := cxf.EditableField{FieldType: fieldType, Value: string(value), Label: label}
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
		for index := range destination.Credential.Fields {
			original[index] = index
		}
		original[len(original)-1] = -1
		destination.Credential.Fields = append(destination.Credential.Fields, field)
		err := item.ReplaceCustomFields(destination.Index, destination.Credential, original)
		if err == nil {
			applyTag()
		}
		return err
	}
	err = item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Label: section, Fields: []cxf.EditableField{field}})
	if err == nil {
		applyTag()
	}
	return err
}

// validCustomFieldType reports whether value names a supported custom field type.
func validCustomFieldType(value string) bool {
	switch value {
	case "text", "hidden", "multiline", "hidden-multiline":
		return true
	default:
		return false
	}
}

// splitFieldPath separates an optional section from a custom field name.
func splitFieldPath(value string) (string, string) {
	section, field, ok := strings.Cut(value, "/")
	if !ok {
		return "", value
	}
	return section, field
}

// editLogin prompts for the fields stored in a CXF Login item.
func (cmd *EditCmd) editLogin(item *model.Item) error {
	if item.Scope == nil {
		item.Scope = &cxf.CredentialScope{URLs: []string{}, AndroidApps: []cxf.AndroidAppID{}}
	}
	url := ""
	if len(item.Scope.URLs) > 0 {
		url = item.Scope.URLs[0]
	}
	url, err := cmd.prompter.askWithDefault("URL", url)
	if err != nil {
		return err
	}
	url, err = normalizeLoginURL(url)
	if err != nil {
		return err
	}
	if len(item.Scope.URLs) == 0 {
		item.Scope.URLs = []string{url}
	} else {
		item.Scope.URLs[0] = url
	}
	credential, index, err := item.BasicAuth()
	if err != nil {
		return err
	}
	username := usernameValue(credential)
	username, err = cmd.prompter.askWithDefault("Username", username)
	if err != nil {
		return err
	}
	changePassword := true
	if cmd.prompter.interactive {
		changePassword, err = cmd.prompter.askYesNo("Change password?", false)
		if err != nil {
			return err
		}
	}
	if credential.Username == nil {
		credential.Username = &cxf.EditableField{FieldType: cxf.FieldTypeString}
	}
	credential.Username.Value = username
	if changePassword {
		password, err := cmd.generatePassword(model.PasswordSettingsFromField(credential.Password).Mode)
		if err != nil {
			return err
		}
		credential.Password, err = model.PasswordField(password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format}, credential.Password)
		if err != nil {
			return err
		}
	}
	if err := item.SetBasicAuth(index, credential); err != nil {
		return err
	}
	note, noteIndex, err := item.Note()
	if err != nil {
		return err
	}
	if cmd.prompter.interactive {
		change, err := cmd.prompter.askYesNo("Change note?", false)
		if err != nil {
			return err
		}
		if change {
			value, err := cmd.prompter.editMultiline("Note", note.Content.Value)
			if err != nil {
				return err
			}
			note.Type = cxf.CredentialTypeNote
			if noteIndex < 0 {
				note.Content.FieldType = cxf.FieldTypeString
			}
			note.Content.Value = value
			if err := item.SetNote(noteIndex, note); err != nil {
				return err
			}
		}
	} else {
		value, err := cmd.prompter.askWithDefault("Note", note.Content.Value)
		if err != nil {
			return err
		}
		if value != "" || noteIndex >= 0 {
			note.Type = cxf.CredentialTypeNote
			if noteIndex < 0 {
				note.Content.FieldType = cxf.FieldTypeString
			}
			note.Content.Value = value
			if err := item.SetNote(noteIndex, note); err != nil {
				return err
			}
		}
	}
	if cmd.prompter.interactive {
		return cmd.editTOTP(item)
	}
	return nil
}

// editTOTP prompts for changes to the item's authenticator credential.
func (cmd *EditCmd) editTOTP(item *model.Item) error {
	credentials, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return err
	}
	totp := cxf.TOTP{}
	if credentials.TOTP != nil {
		totp = *credentials.TOTP
		change, err := cmd.prompter.askYesNo("Change authenticator secret?", false)
		if err != nil {
			return err
		}
		if change {
			secret, err := cmd.prompter.askPassword("Authenticator secret (:clear removes it)")
			if err != nil {
				return err
			}
			if secret == ":clear" {
				item.Credentials = append(item.Credentials[:credentials.TOTPIndex], item.Credentials[credentials.TOTPIndex+1:]...)
				return nil
			}
			if secret != "" {
				totp.Secret = normalizeTOTPSecret(secret)
			}
		}
	} else {
		add, err := cmd.prompter.askYesNo("Add authenticator secret?", false)
		if err != nil || !add {
			return err
		}
		secret, err := cmd.prompter.askPassword("Authenticator secret")
		if err != nil {
			return err
		}
		if secret == "" {
			return nil
		}
		totp = cxf.TOTP{
			Type:      cxf.CredentialTypeTOTP,
			Secret:    normalizeTOTPSecret(secret),
			Algorithm: strings.ToLower(string(paw.TOTPHashDefault)),
			Period:    uint16(paw.TOTPIntervalDefault),
			Digits:    uint16(paw.TOTPDigitsDefault),
		}
	}
	if err := configureTOTP(cmd.prompter, &totp); err != nil {
		return err
	}
	if err := totp.Validate(); err != nil {
		return fmt.Errorf("invalid authenticator secret: %w", err)
	}
	return item.SetTOTP(credentials.TOTPIndex, totp)
}

// normalizeTOTPSecret removes spaces from a Base32 authenticator secret.
func normalizeTOTPSecret(secret string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "-", "").Replace(secret))
}

const (
	totpIntervalMinimum = 5
	totpIntervalMaximum = 60
)

// configureTOTP prompts for optional authenticator settings.
func configureTOTP(prompter *prompter, credential *cxf.TOTP) error {
	change, err := prompter.askYesNo("Change authenticator settings?", false)
	if err != nil || !change {
		return err
	}
	algorithm, err := prompter.askWithDefault("Algorithm (SHA1, SHA256, SHA512)", strings.ToUpper(credential.Algorithm))
	if err != nil {
		return err
	}
	algorithm = strings.ToUpper(strings.TrimSpace(algorithm))
	switch algorithm {
	case string(paw.SHA1), string(paw.SHA256), string(paw.SHA512):
		credential.Algorithm = strings.ToLower(algorithm)
	default:
		return fmt.Errorf("unsupported authenticator algorithm %q", algorithm)
	}
	digits, err := prompter.askWithDefault("Digits (5-10)", strconv.Itoa(int(credential.Digits)))
	if err != nil {
		return err
	}
	digitCount, err := strconv.Atoi(digits)
	if err != nil || digitCount < paw.TOTPDigitsMinimum || digitCount > paw.TOTPDigitsMaximum {
		return fmt.Errorf("authenticator digits must be between %d and %d", paw.TOTPDigitsMinimum, paw.TOTPDigitsMaximum)
	}
	credential.Digits = uint16(digitCount)
	period, err := prompter.askWithDefault("Interval in seconds (5-60)", strconv.Itoa(int(credential.Period)))
	if err != nil {
		return err
	}
	interval, err := strconv.Atoi(period)
	if err != nil || interval < totpIntervalMinimum || interval > totpIntervalMaximum {
		return fmt.Errorf("authenticator interval must be between %d and %d seconds", totpIntervalMinimum, totpIntervalMaximum)
	}
	credential.Period = uint16(interval)
	return nil
}

// editPassword prompts for a replacement password.
func (cmd *EditCmd) editPassword(item *model.Item) error {
	credential, index, err := item.BasicAuth()
	if err != nil {
		return err
	}
	if cmd.prompter.interactive {
		change, err := cmd.prompter.askYesNo("Change password?", false)
		if err != nil || !change {
			return err
		}
	}
	password, err := cmd.generatePassword(model.PasswordSettingsFromField(credential.Password).Mode)
	if err != nil {
		return err
	}
	credential.Password, err = model.PasswordField(password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format}, credential.Password)
	if err != nil {
		return err
	}
	return item.SetBasicAuth(index, credential)
}

// editNote prompts for replacement note content.
func (cmd *EditCmd) editNote(item *model.Item) error {
	note, index, err := item.Note()
	if err != nil {
		return err
	}
	var value string
	if cmd.prompter.interactive {
		change, err := cmd.prompter.askYesNo("Change note?", false)
		if err != nil || !change {
			return err
		}
		value, err = cmd.prompter.editMultiline("Note", note.Content.Value)
		if err != nil {
			return err
		}
	} else {
		value, err = cmd.prompter.askWithDefault("Note", note.Content.Value)
		if err != nil {
			return err
		}
	}
	note.Type = cxf.CredentialTypeNote
	if index < 0 {
		note.Content.FieldType = cxf.FieldTypeString
	}
	note.Content.Value = value
	return item.SetNote(index, note)
}

// editSSHKey updates whether the SSH key should be added to the agent.
func (cmd *EditCmd) editSSHKey(item *model.Item) error {
	addToAgent, err := cmd.prompter.askYesNo("Add to agent", item.Metadata.AddToAgent)
	if err != nil {
		return err
	}
	item.Metadata.AddToAgent = addToAgent
	changePassphrase, err := cmd.prompter.askYesNo("Change key passphrase?", false)
	if err != nil || !changePassphrase {
		return err
	}
	passphrase, err := cmd.prompter.askPassword("New key passphrase (empty removes it)")
	if err != nil {
		return err
	}
	credential, index, err := item.BasicAuth()
	if err != nil {
		return err
	}
	credential.Password, err = model.PasswordField(passphrase, model.PasswordSettingsFromField(credential.Password), credential.Password)
	if err != nil {
		return err
	}
	return item.SetBasicAuth(index, credential)
}

// generatePassword prompts for a password using the supported generators.
func (cmd *EditCmd) generatePassword(defaultMode paw.PasswordMode) (*paw.PasswordSpec, error) {
	return (&PwGenCmd{command: cmd.command}).Pwgen([]paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}, defaultMode)
}
