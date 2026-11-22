// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package cli provides Paw's command-line interface.
package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
	"go.lucor.dev/paw/internal/storage"
)

// Run executes the CLI branch of the application's full argument list.
// The caller handles error reporting and the process exit status.
func Run(args []string, s *storage.FileStorage) error {
	if len(args) < 2 || args[1] != "cli" {
		return invalidArgumentsError("expected paw cli arguments")
	}
	argv := append([]string{"paw cli"}, args[2:]...)
	err := newCommand(s, os.Stdin, os.Stdout, os.Stderr).Run(context.Background(), argv)
	if errors.Is(err, errCanceled) {
		return nil
	}
	return err
}

var errCanceled = errors.New("command canceled")

const (
	// ExitCodeSuccess indicates successful command execution.
	ExitCodeSuccess = 0
	// ExitCodeApplicationError indicates a command failed while running.
	ExitCodeApplicationError = 1
	// ExitCodeInvalidArguments indicates invalid command arguments or flags.
	ExitCodeInvalidArguments = 2
	// ExitCodeInterrupted indicates the process received Ctrl+C.
	ExitCodeInterrupted = 130
)

const vaultReferenceHelp = "VAULT can be a vault name, full ID, or unique ID prefix (at least 7 characters)."

// newCommand builds the root CLI command and its subcommands.
func newCommand(storage *storage.FileStorage, in io.Reader, out, errOut io.Writer) *urfave.Command {
	base := &command{
		storage:     storage,
		prompter:    newPrompter(in, errOut),
		out:         out,
		errOut:      errOut,
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
		readFile: os.ReadFile,
		getenv:   os.Getenv,
		now:      time.Now,
		generateSSHKey: func() (*generatedSSHKey, error) {
			key, err := sshkey.GenerateKey()
			if err != nil {
				return nil, err
			}
			return &generatedSSHKey{PrivateKey: string(key.MarshalPrivateKey()), PublicKey: string(key.MarshalPublicKey()), Fingerprint: key.Fingerprint()}, nil
		},
	}
	return &urfave.Command{
		OnUsageError: onUsageError,
		Name:         "paw cli",
		Usage:        "The CLI application for Paw",
		Action:       requireSubcommand,
		Reader:       in,
		Writer:       out,
		ErrWriter:    errOut,
		// Keep exit handling in main, including errors returned by command.
		ExitErrHandler: leaveExitToCaller,
		Commands: []*urfave.Command{
			newAttachmentCommand(base),
			newAgentCommand(base),
			newAddCommand(base),
			newEditCommand(base),
			newExportCommand(base),
			newHistoryCommand(base),
			newInitCommand(base),
			newListCommand(base),
			newLockCommand(base),
			newPwGenCommand(base),
			newRemoveCommand(base),
			newRestoreCommand(base),
			newShowCommand(base),
			newSyncCommand(base),
			newUnlockCommand(base),
			newVersionCommand(),
		},
	}
}

// leaveExitToCaller lets main print errors and set the process exit status.
func leaveExitToCaller(context.Context, *urfave.Command, error) {}

// onUsageError classifies errors detected by urfave while parsing input.
func onUsageError(_ context.Context, _ *urfave.Command, err error, _ bool) error {
	return invalidArgumentsError("%w", err)
}

// invalidArgumentsError creates an input error with the invalid-arguments exit code.
func invalidArgumentsError(format string, args ...any) error {
	return urfave.Exit(fmt.Errorf(format, args...), ExitCodeInvalidArguments)
}

// requireSubcommand rejects the root command when no subcommand is supplied.
func requireSubcommand(ctx context.Context, c *urfave.Command) error {
	if c.Args().Present() {
		return invalidArgumentsError("unknown command %q for %s", c.Args().First(), c.FullName())
	}
	if err := urfave.ShowSubcommandHelp(c); err != nil {
		return err
	}
	return invalidArgumentsError("a command is required")
}

// commandGroupAction displays help or reports an unknown command-group subcommand.
func commandGroupAction(_ context.Context, c *urfave.Command) error {
	if c.Args().Present() {
		var commands strings.Builder
		for _, command := range c.Commands {
			if command.Name == "help" {
				continue
			}
			fmt.Fprintf(&commands, "\n  %-10s %s", command.Name, command.Usage)
		}
		return invalidArgumentsError("unknown %s subcommand %q.\n\nUsage:\n  %s <command> [arguments]\n\nCommands:%s\n\nRun '%s --help' for details", c.Name, c.Args().First(), c.FullName(), commands.String(), c.FullName())
	}
	return urfave.ShowSubcommandHelp(c)
}

type itemPath struct {
	itemName  string
	itemType  model.ItemType
	vaultName string
}

// String returns the item path in its user-facing form.
func (i itemPath) String() string {
	return filepath.Join(i.vaultName, i.itemType.String(), i.itemName)
}

type itemPathOptions struct {
	fullPath bool
	wildcard bool
}

// parseItemPath parses and validates a vault and item path.
func parseItemPath(path string, opts itemPathOptions) (itemPath, error) {
	parts := strings.Split(path, "/")
	ip := itemPath{}

	if len(parts) > 3 || (opts.fullPath && len(parts) != 3) {
		return ip, invalidArgumentsError("invalid vault item path. Got %q, expected VAULT/ITEM_TYPE/ITEM_NAME", path)
	}

	for i, v := range parts {
		if opts.fullPath && v == "" {
			return ip, invalidArgumentsError("a path element is empty. Got %s, expected VAULT/ITEM_TYPE/ITEM_NAME", path)
		}
		switch i {
		case 0:
			ip.vaultName = v
		case 1:
			var itemType model.ItemType
			var err error
			if opts.wildcard && v == "*" {
				break
			}
			itemType, err = model.ItemTypeFromString(v)
			if err != nil {
				return ip, invalidArgumentsError("%w", err)
			}
			ip.itemType = itemType
		case 2:
			if opts.fullPath && v == "" {
				return ip, invalidArgumentsError("item name cannot be empty")
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
	if sessionID != "" {
		key, err := base.loadVaultKeyWithSession(info.ID, sessionID)
		if err == nil {
			return key, nil
		}
		if _, err := fmt.Fprintln(base.errOut, "[✗] Session is invalid or expired"); err != nil {
			return nil, err
		}
	}

	password, err := base.prompter.askPassword("Enter the vault password")
	if err != nil {
		return nil, err
	}
	return base.storage.LoadVaultKey(info.ID, password)
}

// resolveVaultInfo finds a vault by its display name or filesystem ID.
func (base *command) resolveVaultInfo(value string) (model.VaultInfo, error) {
	vaults, err := base.storage.Vaults()
	if err != nil {
		return model.VaultInfo{}, err
	}
	return resolveVaultInfo(vaults, value)
}

// resolveVaultInfo finds a vault by name, full ID, or unique ID prefix.
func resolveVaultInfo(vaults []model.VaultInfo, value string) (model.VaultInfo, error) {
	matches := make([]model.VaultInfo, 0, 2)
	for _, vault := range vaults {
		id := storage.FormatVaultID(vault.ID)
		if vault.Name == value || (id != "" && (id == value || (len(value) >= 7 && strings.HasPrefix(id, value)))) {
			matches = append(matches, vault)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) == 0 {
		return model.VaultInfo{}, fmt.Errorf("vault %q does not exist", value)
	}
	ids := make([]string, len(matches))
	for i, vault := range matches {
		ids[i] = vault.ID
	}
	prefixes := vaultIDPrefixes(ids)
	lines := make([]string, len(matches))
	for i, vault := range matches {
		lines[i] = fmt.Sprintf("  %s  (%s)", vault.Name, prefixes[ids[i]])
	}
	return model.VaultInfo{}, fmt.Errorf("ambiguous vault %q:\n\n%s\n\nSpecify a Vault ID to continue", value, strings.Join(lines, "\n"))
}

// vaultIDPrefixes assigns each ID its shortest unique prefix of at least seven characters.
func vaultIDPrefixes(ids []string) map[string]string {
	displayIDs := make([]string, len(ids))
	for i, id := range ids {
		displayIDs[i] = storage.FormatVaultID(id)
	}
	prefixes := make(map[string]string, len(ids))
	for i, id := range ids {
		displayID := displayIDs[i]
		length := 7
		for length < len(displayID) {
			unique := true
			for j, other := range displayIDs {
				if i != j && strings.HasPrefix(other, displayID[:length]) {
					unique = false
					break
				}
			}
			if unique {
				break
			}
			length++
		}
		if length > len(displayID) {
			length = len(displayID)
		}
		prefixes[id] = displayID[:length]
	}
	return prefixes
}

// shortVaultID returns the display prefix assigned to an ID.
func shortVaultID(id string) string {
	id = storage.FormatVaultID(id)
	if len(id) > 7 {
		return id[:7]
	}
	return id
}

// loadVaultKeyWithSession loads a vault key, using the agent session when provided.
func (base *command) loadVaultKeyWithSession(vaultName string, sessionID string) (*paw.Key, error) {
	client, err := base.agentClient(base.storage.SocketAgentPath())
	if err != nil {
		return nil, err
	}
	return client.Key(vaultName, sessionID)
}
