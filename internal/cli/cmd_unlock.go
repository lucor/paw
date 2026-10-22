// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"time"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// UnlockCmd unlock a vault and starts a session returning its ID
type UnlockCmd struct {
	*command
	vaultName string
	life      time.Duration
}

func newUnlockCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "unlock",
		Usage:     "Unlocks a vault and returns a session ID",
		ArgsUsage: "VAULT",
		Flags:     []urfave.Flag{&urfave.DurationFlag{Name: "lifetime", Aliases: []string{"t"}, Usage: "Maximum session lifetime (zero means no expiry)"}},
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			return (&UnlockCmd{command: base, vaultName: c.Args().First(), life: c.Duration("lifetime")}).Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *UnlockCmd) Run(ctx context.Context) error {
	base := cmd.command
	s := base.storage
	c, err := base.agentClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}

	key, err := cmd.loadVaultKey(cmd.vaultName, base.getenv(paw.ENV_SESSION))
	if err != nil {
		return err
	}

	sessionID, err := c.Unlock(cmd.vaultName, key, cmd.life)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, "adding SSH keys to the agent...")
	err = cmd.addSSHKeysToAgent(c, key)
	if err != nil {
		fmt.Fprintln(cmd.out, "could not add SSH keys to the agent:", err)
	}
	fmt.Fprintln(cmd.out, "Session ID: ", sessionID)
	fmt.Fprintln(cmd.out, "[✓] vault unlocked")
	return nil
}

func (cmd *UnlockCmd) addSSHKeysToAgent(c agent.PawAgent, key *paw.Key) error {
	s := cmd.storage
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
			fmt.Fprintf(cmd.out, "could not add SSH key to agent. Error: %q - Public key: %s", err, k.MarshalPublicKey())
			return true
		}
		fmt.Fprintf(cmd.out, "added: %s", k.MarshalPublicKey())
		return true
	})
	return nil
}
