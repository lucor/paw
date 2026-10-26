// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"fmt"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// Add adds an item to the vault.
type AddCmd struct {
	*command
	sessionID string
	itemPath
	importPath string
}

func newAddCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name: "add", Usage: "Adds an item to the vault", ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags: []urfave.Flag{sessionFlag(), &urfave.StringFlag{Name: "input", Aliases: []string{"i"}, Usage: "Import an SSH key from FILE"}},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
			if err != nil {
				return err
			}
			return (&AddCmd{command: base, itemPath: path, sessionID: c.String("session"), importPath: c.String("input")}).Run(ctx)
		}),
	}
}

// Run creates and stores a CXF item.
func (cmd *AddCmd) Run(ctx context.Context) error {
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
	case paw.NoteItemType:
		content, err := cmd.prompter.ask("Note")
		if err != nil {
			return err
		}
		created, createErr := model.NewNote(cmd.itemName, content)
		if createErr != nil {
			return createErr
		}
		item = created
	case paw.PasswordItemType:
		password, err := cmd.generatePassword(key, paw.CustomPassword)
		if err != nil {
			return err
		}
		created, createErr := model.NewPassword(cmd.itemName, password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format})
		if createErr != nil {
			return createErr
		}
		item = created
	case paw.LoginItemType:
		item, err = cmd.newLogin(key)
	case paw.SSHKeyItemType:
		item, err = cmd.newSSHKey()
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}
	if err != nil {
		return err
	}
	if err := s.StoreItem(vault, item); err != nil {
		return err
	}
	appState.Modified = cmd.now().UTC()
	if err := s.StoreAppState(appState); err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] item %q added\n", cmd.itemName)
	return nil
}

// newLogin prompts for login fields and builds a CXF item.
func (cmd *AddCmd) newLogin(key *paw.Key) (*model.Item, error) {
	url, err := cmd.prompter.ask("URL")
	if err != nil {
		return nil, err
	}
	username, err := cmd.prompter.ask("Username")
	if err != nil {
		return nil, err
	}
	password, err := cmd.generatePassword(key, paw.CustomPassword)
	if err != nil {
		return nil, err
	}
	item, err := model.NewLogin(cmd.itemName, username, password.Value, model.PasswordSettings{Mode: password.Mode, Length: password.Length, Format: password.Format})
	if err != nil {
		return nil, err
	}
	item.Scope = &cxf.CredentialScope{URLs: []string{url}, AndroidApps: []cxf.AndroidAppID{}}
	note, err := cmd.prompter.ask("Note")
	if err != nil {
		return nil, err
	}
	if note != "" {
		if err := setNoteCredential(item, -1, note); err != nil {
			return nil, err
		}
	}
	return item, nil
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
	if key, ok := privateKey.(*ed25519.PrivateKey); ok {
		privateKey = *key
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("cannot encode SSH private key")
	}
	credential := cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: publicKey.Type(), KeyComment: cmd.itemName, PrivateKey: base64.RawURLEncoding.EncodeToString(der)}
	item, err := model.NewSSHKey(cmd.itemName, credential, passphrase, model.PasswordSettings{Mode: paw.CustomPassword})
	if err != nil {
		return nil, err
	}
	item.Metadata.AddToAgent = addToAgent
	return item, nil
}

// generatePassword prompts for a password using the supported generators.
func (cmd *AddCmd) generatePassword(key *paw.Key, defaultMode paw.PasswordMode) (*paw.Password, error) {
	return (&PwGenCmd{command: cmd.command}).Pwgen(key, []paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}, defaultMode)
}
