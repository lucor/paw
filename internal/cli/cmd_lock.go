// Copyright 2023 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cli

import (
	"fmt"
	"os"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// Lock locks a Paw vault removing all the associated sessions from the agent
type LockCmd struct {
	vaultName string
}

// Name returns the one word command name
func (cmd *LockCmd) Name() string {
	return "lock"
}

// Description returns the command description
func (cmd *LockCmd) Description() string {
	return "Lock a vault"
}

// Usage displays the command usage
func (cmd *LockCmd) Usage() {
	template := `Usage: paw cli lock [OPTION] VAULT

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *LockCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	cmd.vaultName = flagSet.Arg(0)
	return nil
}

// Run runs the command
func (cmd *LockCmd) Run(s paw.Storage) error {
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}
	err = c.Lock(cmd.vaultName)
	if err != nil {
		return err
	}

	fmt.Println("Removing SSH keys from the agent...")
	err = cmd.removeSSHKeysFromAgent(c, s)
	if err != nil {
		fmt.Println("could not remove SSH keys from the agent:", err)
	}
	fmt.Println("[✓] vault locked")
	return nil
}

func (cmd *LockCmd) removeSSHKeysFromAgent(c agent.PawAgent, s paw.Storage) error {
	os.Setenv(paw.ENV_SESSION, "")
	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}
	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return err
	}
	vault.Range(func(id string, meta *paw.Metadata) bool {
		item, err := s.LoadItem(vault, meta)
		if err != nil {
			return false
		}
		if item.GetMetadata().Type != paw.SSHKeyItemType {
			return true
		}
		v := item.(*paw.SSHKey)
		if !v.AddToAgent {
			return true
		}
		k, err := sshkey.ParseKey([]byte(v.PrivateKey))
		if err != nil {
			return true
		}

		err = c.RemoveSSHKey(k.PublicKey())
		if err != nil {
			fmt.Printf("Could not remove SSH key from the agent. Error: %q - Public key: %s", err, k.MarshalPublicKey())
			return true
		}
		fmt.Printf("Removed key: %s", k.MarshalPublicKey())
		return true
	})
	return nil
}
