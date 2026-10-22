// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/paw"
)

// Edit edits an item into the vault
type EditCmd struct {
	*command
	sessionID string
	itemPath
}

func newEditCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "edit",
		Usage:     "Edits an item in the vault",
		ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags:     []urfave.Flag{sessionFlag()},
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

// Run runs the command
func (cmd *EditCmd) Run(ctx context.Context) error {
	base := cmd.command
	s := base.storage
	appState, err := s.LoadAppState()
	if err != nil {
		return err
	}

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

	switch cmd.itemType {
	case paw.LoginItemType:
		err = cmd.editLoginItem(vault.Key(), item)
	case paw.NoteItemType:
		err = cmd.editNoteItem(item)
	case paw.PasswordItemType:
		err = cmd.editPasswordItem(vault.Key(), item)
	case paw.SSHKeyItemType:
		err = cmd.editSSHKeyItem(item)
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}

	if err != nil {
		return err
	}

	now := base.now().UTC()

	item.GetMetadata().Modified = now

	err = s.StoreItem(vault, item)
	if err != nil {
		return err
	}

	err = vault.AddItem(item)
	if err != nil {
		return err
	}

	vault.Modified = now
	err = s.StoreVault(vault)
	if err != nil {
		return err
	}

	appState.Modified = now
	err = s.StoreAppState(appState)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.out, "[✓] item %q modified\n", cmd.itemName)
	return nil
}

func (cmd *EditCmd) editLoginItem(key *paw.Key, item paw.Item) error {
	v := item.(*paw.Login)

	url, err := cmd.prompter.askWithDefault("URL", v.URL.String())
	if err != nil {
		return err
	}
	v.URL.Set(url)

	username, err := cmd.prompter.askWithDefault("Username", v.Username)
	if err != nil {
		return err
	}
	v.Username = username

	pwgenCmd := &PwGenCmd{command: cmd.command}
	modes := []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	}
	password, err := pwgenCmd.Pwgen(key, modes, v.Mode)
	if err != nil {
		return err
	}
	v.Password.Value = password.Value
	v.Password.Mode = password.Mode
	v.Password.Format = password.Format
	v.Password.Length = password.Length

	note, err := cmd.prompter.askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}
	v.Note.Value = note

	item = v
	return nil
}

func (cmd *EditCmd) editNoteItem(item paw.Item) error {
	v := item.(*paw.Note)

	note, err := cmd.prompter.askWithDefault("Note", v.Value)
	if err != nil {
		return err
	}
	v.Value = note

	item = v
	return nil
}

func (cmd *EditCmd) editPasswordItem(key *paw.Key, item paw.Item) error {
	v := item.(*paw.Password)

	pwgenCmd := &PwGenCmd{command: cmd.command}
	modes := []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	}
	password, err := pwgenCmd.Pwgen(key, modes, v.Mode)
	if err != nil {
		return err
	}
	v.Value = password.Value
	v.Mode = password.Mode
	v.Format = password.Format
	v.Length = password.Length

	note, err := cmd.prompter.askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}

func (cmd *EditCmd) editSSHKeyItem(item paw.Item) error {
	v := item.(*paw.SSHKey)

	addToAgent, err := cmd.prompter.askYesNo("Add to agent", v.AddToAgent)
	if err != nil {
		return err
	}
	v.AddToAgent = addToAgent

	note, err := cmd.prompter.askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}
