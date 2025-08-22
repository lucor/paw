// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"fmt"
	"os"
	"time"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// UnlockCmd unlock a vault and starts a session returning its ID
type UnlockCmd struct {
	vaultName string
	life      time.Duration
}

// Name returns the one word command name
func (cmd *UnlockCmd) Name() string {
	return "unlock"
}

// Description returns the command description
func (cmd *UnlockCmd) Description() string {
	return "Unlock a vault returning a session ID"
}

// Usage displays the command usage
func (cmd *UnlockCmd) Usage() {
	template := `Usage: paw cli session [OPTION] COMMAND VAULT

{{ . }}

Options:
  -t, --lifetime=DURATION   Sets the maximum lifetime for the session. Default to never expire
  -h, --help            Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *UnlockCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flagSet.DurationVar(&cmd.life, "t", 0, "")
	flagSet.DurationVar(&cmd.life, "lifetime", 0, "")

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	cmd.vaultName = flagSet.Arg(0)
	return nil
}

// Run runs the command
func (cmd *UnlockCmd) Run(s paw.Storage) error {
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}

	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}

	sessionID, err := c.Unlock(cmd.vaultName, key, cmd.life)
	if err != nil {
		return err
	}

	fmt.Println("adding SSH keys to the agent...")
	err = cmd.addSSHKeysToAgent(c, s, key)
	if err != nil {
		fmt.Println("could not add SSH keys to the agent:", err)
	}
	fmt.Println("Session ID: ", sessionID)
	fmt.Println("[✓] vault unlocked")
	return nil
}

func (cmd *UnlockCmd) addSSHKeysToAgent(c agent.PawAgent, s paw.Storage, key *paw.Key) error {
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

		err = c.AddSSHKey(k.PrivateKey(), v.Comment)
		if err != nil {
			fmt.Printf("could not add SSH key to agent. Error: %q - Public key: %s", err, k.MarshalPublicKey())
			return true
		}
		fmt.Printf("added: %s", k.MarshalPublicKey())
		return true
	})
	return nil
}
