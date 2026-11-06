// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// Edit edits an item in the vault.
type EditCmd struct {
	*command
	sessionID string
	itemPath
}

func newEditCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name: "edit", Usage: "Edits an item in the vault", ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags: []urfave.Flag{sessionFlag()},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
			if err != nil {
				return err
			}
			return (&EditCmd{command: base, itemPath: path, sessionID: c.String("session")}).Run(ctx)
		}),
	}
}

// Run loads, updates, and stores a CXF item.
func (cmd *EditCmd) Run(ctx context.Context) error {
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
		fmt.Fprintf(cmd.out, "[✓] item %q unchanged\n", cmd.itemName)
		return nil
	}
	if err := s.StoreItem(vault, updated); err != nil {
		return err
	}
	appState.Modified = time.Unix(int64(now), 0).UTC()
	if err := s.StoreAppState(appState); err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] item %q modified\n", cmd.itemName)
	return nil
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
	item.Scope.URLs = []string{url}
	credential, index, err := item.BasicAuth()
	if err != nil {
		return err
	}
	username := usernameValue(credential)
	username, err = cmd.prompter.askWithDefault("Username", username)
	if err != nil {
		return err
	}
	password, err := cmd.generatePassword(model.PasswordSettingsFromField(credential.Password).Mode)
	if err != nil {
		return err
	}
	credential.Username = &cxf.EditableField{FieldType: cxf.FieldTypeString, Value: username}
	credential.Password, err = model.PasswordField(password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format}, credential.Password)
	if err != nil {
		return err
	}
	if err := item.SetBasicAuth(index, credential); err != nil {
		return err
	}
	note, noteIndex, err := item.Note()
	if err != nil {
		return err
	}
	value := ""
	if note.Content.Value != "" {
		value = note.Content.Value
	}
	value, err = cmd.prompter.askWithDefault("Note", value)
	if err != nil {
		return err
	}
	if value != "" || noteIndex >= 0 {
		note.Type = cxf.CredentialTypeNote
		note.Content.FieldType = cxf.FieldTypeString
		note.Content.Value = value
		return item.SetNote(noteIndex, note)
	}
	return nil
}

// editPassword prompts for a replacement password.
func (cmd *EditCmd) editPassword(item *model.Item) error {
	credential, index, err := item.BasicAuth()
	if err != nil {
		return err
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
	value := note.Content.Value
	value, err = cmd.prompter.askWithDefault("Note", value)
	if err != nil {
		return err
	}
	note.Type = cxf.CredentialTypeNote
	note.Content.FieldType = cxf.FieldTypeString
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
	return nil
}

// generatePassword prompts for a password using the supported generators.
func (cmd *EditCmd) generatePassword(defaultMode paw.PasswordMode) (*paw.PasswordSpec, error) {
	return (&PwGenCmd{command: cmd.command}).Pwgen([]paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}, defaultMode)
}
