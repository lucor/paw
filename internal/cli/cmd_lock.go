// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/paw"
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
	info, err := base.resolveVaultInfo(cmd.vaultName)
	if err != nil {
		return err
	}
	base.vaultID = info.ID
	c, err := base.agentClient(s.SocketAgentPath())
	if err != nil {
		return fmt.Errorf("agent not available: %w", err)
	}
	err = c.Lock(base.vaultID)
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
	vault, err := cmd.loadVault(key)
	if err != nil {
		return err
	}
	summaries, err := vault.FilterItems("", paw.SSHKeyItemType)
	if err != nil {
		return err
	}
	for _, summary := range summaries {
		item, err := s.LoadItem(vault, summary.ID)
		if err != nil {
			return err
		}
		if !item.Metadata.AddToAgent {
			continue
		}
		credential, err := sshCredential(item)
		if err != nil {
			return err
		}
		_, publicKey, err := sshAgentKey(credential)
		if err != nil {
			return err
		}
		if err := c.RemoveSSHKey(publicKey); err != nil {
			fmt.Fprintf(cmd.out, "Could not remove SSH key from the agent. Error: %q - Public key: %s", err, ssh.MarshalAuthorizedKey(publicKey))
			continue
		}
		fmt.Fprintf(cmd.out, "Removed key: %s", ssh.MarshalAuthorizedKey(publicKey))
	}
	return nil
}
