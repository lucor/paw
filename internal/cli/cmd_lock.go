// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// Lock locks a Paw vault removing all the associated sessions from the agent
type LockCmd struct {
	*command
	vaultName string
}

func newLockCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "lock",
		Usage:     "Locks a vault",
		ArgsUsage: "VAULT",
		Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			return (&LockCmd{command: base, vaultName: c.Args().First()}).Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *LockCmd) Run(ctx context.Context) error {
	base := cmd.command
	s := base.storage
	c, err := base.agentClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}
	err = c.Lock(cmd.vaultName)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, "Removing SSH keys from the agent...")
	err = cmd.removeSSHKeysFromAgent(c)
	if err != nil {
		fmt.Fprintln(cmd.out, "could not remove SSH keys from the agent:", err)
	}
	fmt.Fprintln(cmd.out, "[✓] vault locked")
	return nil
}

func (cmd *LockCmd) removeSSHKeysFromAgent(c agent.PawAgent) error {
	s := cmd.storage
	key, err := cmd.loadVaultKey(cmd.vaultName, "")
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
			fmt.Fprintf(cmd.out, "Could not remove SSH key from the agent. Error: %q - Public key: %s", err, k.MarshalPublicKey())
			return true
		}
		fmt.Fprintf(cmd.out, "Removed key: %s", k.MarshalPublicKey())
		return true
	})
	return nil
}
