// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"crypto"
	"fmt"
	"strings"

	urfave "github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
)

// newAddSSHKeyCommand registers add ssh-key.
func newAddSSHKeyCommand(base *command) *urfave.Command {
	cmd := &AddCmd{command: base, itemPath: itemPath{itemType: model.SSHKeyItemType}}
	return &urfave.Command{
		Name:        "ssh-key",
		Usage:       "Add a " + model.SSHKeyItemType.Label() + " item",
		ArgsUsage:   "VAULT NAME",
		Description: vaultReferenceHelp,
		Action:      cmd.Run,
		Flags: []urfave.Flag{
			&urfave.StringFlag{Name: "input", Usage: "Import an SSH private key from FILE"},
			&urfave.BoolFlag{Name: "add-to-agent", Usage: "Load the key into the SSH agent when the vault is unlocked"},
		},
		OnUsageError:  onUsageError,
		ArgValidator:  completionArgValidator(cmd.validateSSHKey),
		ShellComplete: base.completeVaults(false),
	}
}

// validateSSHKey checks the arguments and the optional private key file path.
func (cmd *AddCmd) validateSSHKey(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := cmd.validateItem(c); err != nil {
		return ctx, err
	}
	cmd.importPath = c.String("input")
	if c.IsSet("input") && (strings.TrimSpace(cmd.importPath) == "" || cmd.importPath == "-") {
		return ctx, invalidArgumentsError("--input requires a private key file path")
	}
	cmd.addToAgent = c.Bool("add-to-agent")
	cmd.addToAgentSet = c.IsSet("add-to-agent")
	return ctx, nil
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
	if _, err := fmt.Fprintln(cmd.errOut, "The key fingerprint is:"); err != nil {
		return nil, err
	}
	if _, err := fmt.Fprintln(cmd.errOut, ssh.FingerprintSHA256(publicKey)); err != nil {
		return nil, err
	}
	addToAgent := cmd.addToAgent
	if cmd.wizard || !cmd.addToAgentSet && cmd.prompter.interactive {
		var err error
		addToAgent, err = cmd.prompter.askYesNo("Add to SSH Agent?", false)
		if err != nil {
			return nil, err
		}
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
