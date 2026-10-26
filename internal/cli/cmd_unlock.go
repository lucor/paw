// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"time"

	urfave "github.com/urfave/cli/v3"
	"golang.org/x/crypto/ssh"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
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

	vault, err := base.loadVault(key)
	if err != nil {
		return err
	}
	sessionID, err := c.Unlock(vault.Accounts[0].ID, key, cmd.life)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.out, "adding SSH keys to the agent...")
	err = cmd.addSSHKeysToAgent(c, vault)
	if err != nil {
		fmt.Fprintln(cmd.out, "could not add SSH keys to the agent:", err)
	}
	fmt.Fprintln(cmd.out, "Session ID: ", sessionID)
	fmt.Fprintln(cmd.out, "[✓] vault unlocked")
	return nil
}

// addSSHKeysToAgent loads configured SSH items and adds their keys to the agent.
func (cmd *UnlockCmd) addSSHKeysToAgent(c agent.PawAgent, vault *model.Vault) error {
	summaries, err := vault.FilterItems("", paw.SSHKeyItemType)
	if err != nil {
		return err
	}
	for _, summary := range summaries {
		item, err := cmd.storage.LoadItem(vault, summary.ID)
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
		privateKey, publicKey, err := sshAgentKey(credential)
		if err != nil {
			return err
		}
		if err := c.AddSSHKey(privateKey, credential.KeyComment); err != nil {
			fmt.Fprintf(cmd.out, "could not add SSH key to agent. Error: %q - Public key: %s", err, ssh.MarshalAuthorizedKey(publicKey))
			continue
		}
		fmt.Fprintf(cmd.out, "added: %s", ssh.MarshalAuthorizedKey(publicKey))
	}
	return nil
}
