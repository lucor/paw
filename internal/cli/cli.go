// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
	"go.lucor.dev/paw/internal/storage"
)

// Run executes the CLI branch of the application's full argument list.
// The caller handles error reporting and the process exit status.
func Run(args []string, s storage.Storage) error {
	if len(args) < 2 || args[1] != "cli" {
		return fmt.Errorf("expected paw cli arguments")
	}
	argv := append([]string{"paw cli"}, args[2:]...)
	err := newCommand(s).Run(context.Background(), argv)
	if errors.Is(err, errCanceled) {
		return nil
	}
	return err
}

var errCanceled = errors.New("command canceled")

func newCommand(storage storage.Storage) *urfave.Command {
	base := &command{
		storage:     storage,
		agentClient: agent.NewClient,
		startAgent: func(socketPath string) error {
			a := agent.NewCLI()
			defer a.Close()
			agent.Run(a, socketPath)
			return nil
		},
		clipboard: func(ctx context.Context, data []byte) error {
			if err := initClipboard(); err != nil {
				return err
			}
			return writeToClipboard(ctx, data)
		},
		readFile:   os.ReadFile,
		getenv:     os.Getenv,
		now:        time.Now,
		oneTimeKey: paw.MakeOneTimeKey,
		generateSSHKey: func() (*paw.SSHKey, error) {
			key, err := sshkey.GenerateKey()
			if err != nil {
				return nil, err
			}
			item := paw.NewSSHKey()
			item.PrivateKey = string(key.MarshalPrivateKey())
			item.PublicKey = string(key.MarshalPublicKey())
			item.Fingerprint = key.Fingerprint()
			return item, nil
		},
	}
	return &urfave.Command{
		Name:   "paw cli",
		Usage:  "The CLI application for Paw",
		Action: requireSubcommand,
		// Keep exit handling in main, including errors returned by command.
		ExitErrHandler: func(context.Context, *urfave.Command, error) {},
		Commands: []*urfave.Command{
			newAgentCommand(base),
			newAddCommand(base),
			newEditCommand(base),
			newInitCommand(base),
			newListCommand(base),
			newLockCommand(base),
			newPwGenCommand(base),
			newRemoveCommand(base),
			newShowCommand(base),
			newUnlockCommand(base),
			newVersionCommand(),
		},
	}
}

func requireSubcommand(ctx context.Context, c *urfave.Command) error {
	if c.Args().Present() {
		return fmt.Errorf("unknown command %q for %s", c.Args().First(), c.FullName())
	}
	if err := urfave.ShowSubcommandHelp(c); err != nil {
		return err
	}
	return fmt.Errorf("a command is required")
}

func withArgs(min, max int, action urfave.ActionFunc) urfave.ActionFunc {
	return func(ctx context.Context, c *urfave.Command) error {
		if n := c.Args().Len(); n < min || n > max {
			return fmt.Errorf("invalid arguments; usage: %s %s", c.FullName(), strings.TrimSpace(c.ArgsUsage))
		}
		return action(ctx, c)
	}
}

type itemPath struct {
	itemName  string
	itemType  paw.ItemType
	vaultName string
}

func (i itemPath) String() string {
	return filepath.Join(i.vaultName, i.itemType.String(), i.itemName)
}

type itemPathOptions struct {
	fullPath bool
	wildcard bool
}

func parseItemPath(path string, opts itemPathOptions) (itemPath, error) {
	parts := strings.Split(path, "/")
	ip := itemPath{}

	if len(parts) > 3 || (opts.fullPath && len(parts) != 3) {
		return ip, fmt.Errorf("invalid vault item path. Got %q, expected VAULT_NAME/ITEM_TYPE/ITEM_NAME", path)
	}

	for i, v := range parts {
		if opts.fullPath && v == "" {
			return ip, fmt.Errorf("a path element is empty. Got %s, expected VAULT_NAME/ITEM_TYPE/ITEM_NAME", path)
		}
		switch i {
		case 0:
			ip.vaultName = v
		case 1:
			var itemType paw.ItemType
			var err error
			if opts.wildcard && v == "*" {
				break
			}
			itemType, err = paw.ItemTypeFromString(v)
			if err != nil {
				return ip, err
			}
			ip.itemType = itemType
		case 2:
			if opts.fullPath && v == "" {
				return ip, fmt.Errorf("item name cannot be empty")
			}
			ip.itemName = v
		}
	}
	return ip, nil
}

// loadVaultKey returns the key to unlock the vault from the storage
// it will use the supplied session if set,
// otherwise will ask for the vault's password
func (base *command) loadVaultKey(vaultName, sessionID string) (*paw.Key, error) {
	info, err := base.resolveVaultInfo(vaultName)
	if err != nil {
		return nil, err
	}
	base.vaultID = info.ID
	base.vaultNeedsMigration = info.NeedsMigration
	if sessionID != "" {
		key, err := base.loadVaultKeyWithSession(info.ID, sessionID)
		if err == nil {
			return key, nil
		}
		fmt.Fprintln(base.errOut, "[✗] Session is invalid or expired")
	}

	password, err := base.prompter.askPassword("Enter the vault password")
	if err != nil {
		return nil, err
	}
	return base.storage.LoadVaultKey(info.ID, password)
}

// resolveVaultInfo finds a vault by its display name or filesystem ID.
func (base *command) resolveVaultInfo(value string) (storage.VaultInfo, error) {
	vaults, err := base.storage.Vaults()
	if err != nil {
		return storage.VaultInfo{}, err
	}
	return storage.FindVaultInfo(vaults, value)
}

func (base *command) loadVaultKeyWithSession(vaultName string, sessionID string) (*paw.Key, error) {
	client, err := base.agentClient(base.storage.SocketAgentPath())
	if err != nil {
		return nil, err
	}
	return client.Key(vaultName, sessionID)
}
