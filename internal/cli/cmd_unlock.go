// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"time"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// UnlockCmd unlock a vault and starts a session returning its ID
type UnlockCmd struct {
	*command
	vaultName string
	life      time.Duration
}

// newUnlockCommand constructs the vault-unlock command.
func newUnlockCommand(base *command) *urfave.Command {
	cmd := &UnlockCmd{command: base}
	c := &urfave.Command{
		Name:         "unlock",
		Usage:        "Unlocks a vault and returns a session ID",
		ArgsUsage:    "VAULT",
		Description:  vaultReferenceHelp,
		Flags:        []urfave.Flag{&urfave.DurationFlag{Name: "lifetime", Aliases: []string{"t"}, Usage: "Maximum session lifetime (zero means no expiry)"}},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the vault argument and captures the requested lifetime.
func (cmd *UnlockCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 1, 1); err != nil {
		return ctx, err
	}
	cmd.vaultName = c.Args().First()
	cmd.life = c.Duration("lifetime")
	if cmd.life < 0 {
		return ctx, invalidArgumentsError("lifetime cannot be negative")
	}
	return ctx, nil
}

// Run unlocks the selected vault and restores its agent keys when configured.
func (cmd *UnlockCmd) Run(ctx context.Context, _ *urfave.Command) error {
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
	if key.Migrated() {
		if err := base.Infof("Upgraded vault encryption to post-quantum age.\n"); err != nil {
			return err
		}
	}

	vault, err := base.loadVault(key)
	if err != nil {
		return err
	}
	sessionID, err := c.Unlock(vault.Accounts[0].ID, key, cmd.life)
	if err != nil {
		return err
	}
	err = cmd.addSSHKeysToAgent(c, vault)
	if err != nil {
		if writeErr := base.Warnf("could not load SSH keys for the agent: %v\n", err); writeErr != nil {
			return writeErr
		}
	}
	return base.Printf("%s\n", sessionID)
}

// addSSHKeysToAgent loads configured SSH items and adds their keys to the agent.
func (cmd *UnlockCmd) addSSHKeysToAgent(c agent.PawAgent, vault *model.Vault) error {
	summaries, err := vault.FilterItems("", model.SSHKeyItemType)
	if err != nil {
		return err
	}
	for _, summary := range summaries {
		item, err := cmd.storage.LoadItem(vault, summary.ID)
		if err != nil {
			if err := cmd.Warnf("could not load SSH key %q: %v\n", summary.Title, err); err != nil {
				return err
			}
			continue
		}
		if !item.Metadata.AddToAgent {
			continue
		}
		credential, err := item.SSHKey()
		if err != nil {
			if err := cmd.Warnf("could not read SSH key %q: %v\n", summary.Title, err); err != nil {
				return err
			}
			continue
		}
		privateKey, _, err := model.ParseSSHKey(credential)
		if err != nil {
			if err := cmd.Warnf("could not parse SSH key %q: %v\n", summary.Title, err); err != nil {
				return err
			}
			continue
		}
		if err := c.AddSSHKey(privateKey, credential.KeyComment); err != nil {
			if err := cmd.Warnf("could not add SSH key %q to the agent: %v\n", summary.Title, err); err != nil {
				return err
			}
			continue
		}
	}
	return nil
}
