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

func newHistoryCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "history",
		Usage:     "Lists and restores item history",
		ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME",
		Flags:     []urfave.Flag{sessionFlag()},
		Action: func(ctx context.Context, c *urfave.Command) error {
			if !c.Args().Present() {
				return urfave.ShowSubcommandHelp(c)
			}
			if c.Args().Len() != 1 {
				return fmt.Errorf("unexpected arguments; usage: %s <VAULT_NAME/ITEM_TYPE/ITEM_NAME>", c.FullName())
			}
			if !strings.Contains(c.Args().First(), "/") {
				return fmt.Errorf("unknown history subcommand %q; use 'list', 'show', or 'restore'", c.Args().First())
			}
			return runHistoryList(base, ctx, c)
		},
		Commands: []*urfave.Command{
			{Name: "list", Usage: "Lists item history", ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME", Action: withArgs(1, 1, func(ctx context.Context, c *urfave.Command) error {
				return runHistoryList(base, ctx, c)
			})},
			{Name: "show", Usage: "Shows a historical item version", ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME INDEX", Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
				cmd, err := newHistoryCmd(base, c)
				if err != nil {
					return err
				}
				index, err := historyIndex(c.Args().Get(1))
				if err != nil {
					return err
				}
				return cmd.show(ctx, index)
			})},
			{Name: "restore", Usage: "Restores a historical item version", ArgsUsage: "VAULT_NAME/ITEM_TYPE/ITEM_NAME INDEX", Action: withArgs(2, 2, func(ctx context.Context, c *urfave.Command) error {
				cmd, err := newHistoryCmd(base, c)
				if err != nil {
					return err
				}
				index, err := historyIndex(c.Args().Get(1))
				if err != nil {
					return err
				}
				return cmd.restore(index)
			})},
		},
	}
}

func runHistoryList(base *command, ctx context.Context, c *urfave.Command) error {
	cmd, err := newHistoryCmd(base, c)
	if err != nil {
		return err
	}
	return cmd.list()
}

type historyCmd struct {
	*command
	sessionID string
	itemPath
}

func newHistoryCmd(base *command, c *urfave.Command) (*historyCmd, error) {
	path, err := parseItemPath(c.Args().First(), itemPathOptions{fullPath: true})
	if err != nil {
		return nil, err
	}
	return &historyCmd{command: base.forInvocation(c), sessionID: c.String("session"), itemPath: path}, nil
}

func historyIndex(value string) (int, error) {
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return 0, fmt.Errorf("invalid history index %q", value)
	}
	return index, nil
}

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
		fmt.Fprintf(cmd.out, "[✓] item %q is already at that version\n", cmd.itemName)
		return nil
	}
	appState.Modified = now
	if err := cmd.storage.StoreAppState(appState); err != nil {
		return err
	}
	fmt.Fprintf(cmd.out, "[✓] item %q restored from %s\n", cmd.itemName, capturedAt)
	return nil
}
