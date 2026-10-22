// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// Add adds an item to the vault
type AddCmd struct {
	*command
	sessionID string
	itemPath
	importPath string
}

func newAddCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "add",
		Usage:     "Adds an item to the vault",
		ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags: []urfave.Flag{
			sessionFlag(),
			&urfave.StringFlag{Name: "input", Aliases: []string{"i"}, Usage: "Import an SSH key from FILE"},
		},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
			if err != nil {
				return err
			}
			cmd := &AddCmd{command: base, itemPath: path, sessionID: c.String("session"), importPath: c.String("input")}
			return cmd.Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *AddCmd) Run(ctx context.Context) error {
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

	if ok := vault.HasItem(item); ok {
		return fmt.Errorf("item with same name already exists")
	}

	switch cmd.itemType {
	case paw.LoginItemType:
		err = cmd.addLoginItem(vault.Key(), item)
	case paw.NoteItemType:
		err = cmd.addNoteItem(item)
	case paw.PasswordItemType:
		err = cmd.addPasswordItem(vault.Key(), item)
	case paw.SSHKeyItemType:
		err = cmd.addSSHKeyItem(item)
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}

	if err != nil {
		return err
	}

	now := base.now().UTC()
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

	fmt.Fprintf(cmd.out, "[✓] item %q added\n", cmd.itemName)
	return nil
}

func (cmd *AddCmd) addLoginItem(key *paw.Key, item paw.Item) error {
	v := item.(*paw.Login)

	url, err := cmd.prompter.ask("URL")
	if err != nil {
		return err
	}
	v.URL.Set(url)

	username, err := cmd.prompter.ask("Username")
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

	note, err := cmd.prompter.ask("Note")
	if err != nil {
		return err
	}
	v.Note.Value = note

	item = v
	return nil
}

func (cmd *AddCmd) addNoteItem(item paw.Item) error {
	v := item.(*paw.Note)

	note, err := cmd.prompter.ask("Note")
	if err != nil {
		return err
	}
	v.Value = note

	item = v
	return nil
}

func (cmd *AddCmd) addPasswordItem(key *paw.Key, item paw.Item) error {
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

	note, err := cmd.prompter.ask("Note")
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}

func (cmd *AddCmd) addSSHKeyItem(item paw.Item) error {
	v := item.(*paw.SSHKey)

	if cmd.importPath != "" {
		if err := cmd.importSSHKey(v); err != nil {
			return err
		}
		fmt.Fprintln(cmd.out, "The key fingerprint is:")
		fmt.Fprintln(cmd.out, v.Fingerprint)
		answer, err := cmd.prompter.askYesNo("Continue?", true)
		if err != nil {
			return err
		}
		if !answer {
			return errCanceled
		}
	} else {
		generated, err := cmd.generateSSHKey()
		if err != nil {
			return err
		}

		v.PrivateKey = generated.PrivateKey
		v.PublicKey = generated.PublicKey
		v.Fingerprint = generated.Fingerprint

		fmt.Fprintln(cmd.out, "The key fingerprint is:")
		fmt.Fprintln(cmd.out, v.Fingerprint)
	}

	addToAgent, err := cmd.prompter.askYesNo("Add to SSH Agent?", false)
	if err != nil {
		return err
	}
	v.AddToAgent = addToAgent

	note, err := cmd.prompter.ask("Note")
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}

func (cmd *AddCmd) importSSHKey(item *paw.SSHKey) error {
	content, err := cmd.readFile(cmd.importPath)
	if err != nil {
		return err
	}

	sk, err := sshkey.ParseKey(content)
	if err == nil {
		item.PrivateKey = string(sk.MarshalPrivateKey())
		item.PublicKey = string(sk.MarshalPublicKey())
		item.Fingerprint = string(sk.Fingerprint())

		fmt.Fprintln(cmd.out, "[i] importing SSH key with public key:")
		fmt.Fprintln(cmd.out, item.PublicKey)
		return nil
	}
	// Check if SSH Key is protected with a passphrase
	if _, ok := err.(*ssh.PassphraseMissingError); !ok {
		return err
	}
	passphrase, err := cmd.prompter.askPassword("Private Key is password protected")
	if err != nil {
		return err
	}
	sk, err = sshkey.ParseKeyWithPassphrase(content, []byte(passphrase))
	if err != nil {
		return err
	}
	item.Passphrase = &paw.Password{Value: passphrase, Mode: paw.CustomPassword}
	item.PrivateKey = string(sk.MarshalPrivateKey())
	item.PublicKey = string(sk.MarshalPublicKey())
	item.Fingerprint = string(sk.Fingerprint())
	return nil
}
