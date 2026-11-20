// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/model"
)

// newHistoryCommand constructs the item-history command group.
func newHistoryCommand(base *command) *urfave.Command {
	cmd := &HistoryCommand{command: base, operation: historyDefault}
	c := &urfave.Command{
		Name:        "history",
		Usage:       "Lists and restores item history",
		ArgsUsage:   "VAULT/ITEM_TYPE/ITEM_NAME",
		Description: vaultReferenceHelp,
		Flags:       []urfave.Flag{sessionFlag()},
		Commands: []*urfave.Command{
			newHistorySubcommand(base, "list", "Lists item history", historyList),
			newHistorySubcommand(base, "show", "Shows a historical item version", historyShow),
			newHistorySubcommand(base, "restore", "Restores a historical item version", historyRestore),
		},
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

const (
	historyDefault = "default"
	historyList    = "list"
	historyShow    = "show"
	historyRestore = "restore"
)

// HistoryCommand implements history's legacy default listing and subcommands.
type HistoryCommand struct {
	*command
	operation  string
	history    *historyCmd
	index      int
	cliCommand *urfave.Command
}

// newHistorySubcommand builds one history operation command.
func newHistorySubcommand(base *command, name, usage, operation string) *urfave.Command {
	cmd := &HistoryCommand{command: base, operation: operation}
	argsUsage := "VAULT/ITEM_TYPE/ITEM_NAME"
	if operation != historyList {
		argsUsage += " INDEX"
	}
	c := &urfave.Command{
		Name:        name,
		Usage:       usage,
		ArgsUsage:   argsUsage,
		Description: vaultReferenceHelp,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks history arguments and parses the selected item path and index.
func (cmd *HistoryCommand) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	cmd.cliCommand = c
	if cmd.operation == historyDefault {
		if c.Args().Len() == 0 {
			return ctx, nil
		}
		if c.Args().First() == historyList || c.Args().First() == historyShow || c.Args().First() == historyRestore {
			return ctx, nil
		}
		if err := validateArgs(c, 1, 1); err != nil {
			return ctx, fmt.Errorf("unexpected arguments; usage: %s <VAULT/ITEM_TYPE/ITEM_NAME>", c.FullName())
		}
		if !strings.Contains(c.Args().First(), "/") {
			return ctx, fmt.Errorf("unknown history subcommand %q; use 'list', 'show', or 'restore'", c.Args().First())
		}
	} else if cmd.operation == historyList {
		if err := validateArgs(c, 1, 1); err != nil {
			return ctx, err
		}
	} else if err := validateArgs(c, 2, 2); err != nil {
		return ctx, err
	}
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return ctx, err
	}
	cmd.history = &historyCmd{command: cmd.command, sessionID: c.String("session"), itemPath: path}
	if cmd.operation == historyShow || cmd.operation == historyRestore {
		cmd.index, err = historyIndex(c.Args().Get(1))
		if err != nil {
			return ctx, err
		}
	}
	return ctx, nil
}

// Run lists, shows, or restores the selected history version.
func (cmd *HistoryCommand) Run(ctx context.Context, _ *urfave.Command) error {
	if cmd.operation == historyDefault && cmd.history == nil {
		return urfave.ShowSubcommandHelp(cmd.cliCommand)
	}
	switch cmd.operation {
	case historyDefault, historyList:
		return cmd.history.list()
	case historyShow:
		return cmd.history.show(ctx, cmd.index)
	case historyRestore:
		return cmd.history.restore(cmd.index)
	default:
		return fmt.Errorf("unknown history operation %q", cmd.operation)
	}
}

type historyCmd struct {
	*command
	sessionID string
	itemPath
}

// historyIndex parses a zero-based history index.
func historyIndex(value string) (int, error) {
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid history index %q", value)
	}
	return index, nil
}

// load unlocks the vault and returns the addressed item and its history.
func (cmd *historyCmd) load() (*model.Vault, *model.Item, model.History, error) {
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return nil, nil, model.History{}, err
	}
	vault, err := cmd.loadVault(key)
	if err != nil {
		return nil, nil, model.History{}, err
	}
	summary, err := vault.FindItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return nil, nil, model.History{}, err
	}
	item, err := cmd.storage.LoadItem(vault, summary.ID)
	if err != nil {
		return nil, nil, model.History{}, err
	}
	history, err := item.History()
	if err != nil {
		return nil, nil, model.History{}, err
	}
	return vault, item, history, nil
}

// list prints the item's history entries from newest to oldest.
func (cmd *historyCmd) list() error {
	_, _, history, err := cmd.load()
	if err != nil {
		return err
	}
	if len(history.Entries) == 0 {
		_, err := fmt.Fprintln(cmd.out, "No history found")
		return err
	}
	w := tabwriter.NewWriter(cmd.out, 0, 0, 1, ' ', tabwriter.Debug)
	if _, err := fmt.Fprintln(w, "INDEX\tCAPTURED AT"); err != nil {
		return err
	}
	for displayIndex, storedIndex := 0, len(history.Entries)-1; storedIndex >= 0; displayIndex, storedIndex = displayIndex+1, storedIndex-1 {
		capturedAt := time.Unix(int64(history.Entries[storedIndex].CapturedAt), 0).Local().Format(time.RFC1123)
		if _, err := fmt.Fprintf(w, "%d\t%s\n", displayIndex, capturedAt); err != nil {
			return err
		}
	}
	return w.Flush()
}

// selected loads a history entry by its one-based display index.
func (cmd *historyCmd) selected(index int) (*model.Vault, *model.Item, model.HistoryEntry, int, error) {
	vault, item, history, err := cmd.load()
	if err != nil {
		return nil, nil, model.HistoryEntry{}, 0, err
	}
	if len(history.Entries) == 0 {
		return nil, nil, model.HistoryEntry{}, 0, fmt.Errorf("item has no history")
	}
	if index >= len(history.Entries) {
		return nil, nil, model.HistoryEntry{}, 0, fmt.Errorf("history index %d is out of range", index)
	}
	storedIndex := len(history.Entries) - 1 - index
	return vault, item, history.Entries[storedIndex], storedIndex, nil
}

// show displays a selected historical item version.
func (cmd *historyCmd) show(ctx context.Context, index int) error {
	_, _, entry, _, err := cmd.selected(index)
	if err != nil {
		return err
	}
	capturedAt := time.Unix(int64(entry.CapturedAt), 0).Local().Format(time.RFC1123)
	if _, err := fmt.Fprintf(cmd.out, "History: %s\n", capturedAt); err != nil {
		return err
	}
	show := &ShowCmd{command: cmd.command, itemPath: cmd.itemPath}
	var historical model.Item
	data, err := json.Marshal(entry.Item, json.Deterministic(true))
	if err != nil {
		return fmt.Errorf("cannot encode history snapshot: %w", err)
	}
	if err := json.Unmarshal(data, &historical); err != nil {
		return fmt.Errorf("cannot decode history snapshot: %w", err)
	}
	show.itemType = historical.Metadata.Type
	return show.showItem(ctx, &historical)
}

// restore replaces the current item with a selected historical version.
func (cmd *historyCmd) restore(index int) error {
	vault, item, entry, storedIndex, err := cmd.selected(index)
	if err != nil {
		return err
	}
	capturedAt := time.Unix(int64(entry.CapturedAt), 0).Local().Format(time.RFC1123)
	confirmed, err := cmd.prompter.askYesNo(fmt.Sprintf("Restore version from %s? The current version will be saved in History.", capturedAt), false)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(cmd.out, "[✓] restore canceled")
		return nil
	}
	appState, err := cmd.storage.LoadAppState()
	if err != nil {
		return err
	}
	now := cmd.now().UTC()
	_, changed, err := cmd.storage.RestoreItem(vault, item.ID, storedIndex, &entry, uint64(now.Unix()))
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintf(cmd.out, "[✓] item %q is already at that version\n", safeTerminalText(cmd.itemName))
		return nil
	}
	appState.Modified = now
	if err := cmd.storage.StoreAppState(appState); err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] item %q restored from %s\n", safeTerminalText(cmd.itemName), capturedAt)
	return nil
}
