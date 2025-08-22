// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package cli

import (
	"fmt"
	"os"
	"time"

	"lucor.dev/paw/internal/paw"
)

// RemoveCmd removes an item from the vault
type RemoveCmd struct {
	itemPath
}

// Name returns the one word command name
func (cmd *RemoveCmd) Name() string {
	return "rm"
}

// Description returns the command description
func (cmd *RemoveCmd) Description() string {
	return "Removes an item from the vault"
}

// Usage displays the command usage
func (cmd *RemoveCmd) Usage() {
	template := `Usage: paw cli rm [OPTION] VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}

Options:
  -h, --help                  Displays this help and exit
      --session=SESSION_ID    Sets a session ID to use instead of the env var
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *RemoveCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{Session: true})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}
	flags.SetEnv()

	itemPath, err := parseItemPath(flagSet.Arg(0), itemPathOptions{fullPath: true})
	if err != nil {
		return err
	}
	cmd.itemPath = itemPath
	return nil
}

// Run runs the command
func (cmd *RemoveCmd) Run(s paw.Storage) error {
	appState, err := s.LoadAppState()
	if err != nil {
		return err
	}

	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return err
	}

	item, err := paw.NewItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}

	if ok := vault.HasItem(item); !ok {
		return fmt.Errorf("item does not exists")
	}

	msg := fmt.Sprintf("Are you sure you want to delete %q?", cmd.itemPath)
	confirm, err := askYesNo(msg, false)
	if err != nil {
		return err
	}
	if !confirm {
		os.Exit(0)
	}

	err = s.DeleteItem(vault, item)
	if err != nil {
		return err
	}

	vault.DeleteItem(item)

	now := time.Now().UTC()
	vault.Modified = now
	err = s.StoreVault(vault)
	if err != nil {
		return err
	}

	appState.Modified = now
	err = s.StoreAppState(appState)
	if err != nil {
		return err
	}

	fmt.Printf("[✓] item %q removed\n", cmd.itemName)
	return nil
}
