// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestItemHistoryListAndShow(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	history, err := item.History()
	require.NoError(t, err)
	oldest := *item
	oldest.Title = "Oldest"
	setHistoryNote(t, &oldest, "oldest-secret")
	require.NoError(t, history.AddSnapshot(&oldest, 100))
	newest := *item
	newest.Title = "Newest"
	setHistoryNote(t, &newest, "newest-secret")
	require.NoError(t, history.AddSnapshot(&newest, 200))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	path := vault.Accounts[0].ID + "/note/Current"

	listed, err := executeWithHistoryBase(t, base, []string{"history", path})
	require.NoError(t, err)
	require.Contains(t, listed, "INDEX")
	require.Less(t, strings.Index(listed, time.Unix(200, 0).Local().Format(time.RFC1123)), strings.Index(listed, time.Unix(100, 0).Local().Format(time.RFC1123)))
	require.NotContains(t, listed, "oldest-secret")
	require.NotContains(t, listed, "newest-secret")

	shown, err := executeWithHistoryBase(t, base, []string{"history", "show", path, "0"})
	require.NoError(t, err)
	require.Contains(t, shown, "History: "+time.Unix(200, 0).Local().Format(time.RFC1123))
	require.Contains(t, shown, "newest-secret")
	require.NotContains(t, shown, "oldest-secret")
	require.NotContains(t, shown, "current-secret")
	_, err = executeWithHistoryBase(t, base, []string{"history", "show", path, "2"})
	require.ErrorContains(t, err, "out of range")
}

func TestItemHistoryRestoreUsesNewestFirstIndexAndConfirms(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	history, err := item.History()
	require.NoError(t, err)
	oldest := *item
	oldest.Title = "Oldest"
	setHistoryNote(t, &oldest, "oldest-secret")
	require.NoError(t, history.AddSnapshot(&oldest, 100))
	newest := *item
	newest.Title = "Newest"
	setHistoryNote(t, &newest, "newest-secret")
	require.NoError(t, history.AddSnapshot(&newest, 200))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	path := vault.Accounts[0].ID + "/note/Current"

	base.prompter = historyPrompter("n\n")
	canceled, err := executeWithHistoryBase(t, base, []string{"history", "restore", path, "0"})
	require.NoError(t, err)
	require.Contains(t, canceled, "restore canceled")
	current, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", current.Title)

	base.prompter = historyPrompter("y\n")
	base.now = func() time.Time { return time.Unix(300, 0).UTC() }
	restored, err := executeWithHistoryBase(t, base, []string{"history", "restore", path, "0"})
	require.NoError(t, err)
	require.Contains(t, restored, "restored from")
	vault, err = store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	current, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Newest", current.Title)
	currentHistory, err := current.History()
	require.NoError(t, err)
	require.Len(t, currentHistory.Entries, 3)
	require.Equal(t, "Current", currentHistory.Entries[2].Item.Title)
}

func TestItemHistoryWithoutEntriesListsEmptyAndRejectsRestore(t *testing.T) {
	_, vault, _, base := historyCLIFixture(t)
	path := vault.Accounts[0].ID + "/note/Current"
	listed, err := executeWithHistoryBase(t, base, []string{"history", path})
	require.NoError(t, err)
	require.Contains(t, listed, "No history found")
	_, err = executeWithHistoryBase(t, base, []string{"history", "restore", path, "0"})
	require.ErrorContains(t, err, "no history")
}

func historyCLIFixture(t *testing.T) (*storage.FileStorage, *model.Vault, *model.Item, *command) {
	t.Helper()
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault, err := store.CreateVault("Personal", "synthetic-password")
	require.NoError(t, err)
	item, err := model.NewNote("Current", "current-secret")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	base := &command{
		storage:  store,
		prompter: historyPrompter(""),
		now:      time.Now,
	}
	return store, vault, item, base
}

func executeWithHistoryBase(t *testing.T, base *command, args []string) (string, error) {
	t.Helper()
	cliCommand := &urfave.Command{Name: "paw cli", Commands: []*urfave.Command{newHistoryCommand(base)}}
	out, _, err := executeWithInput(t, args, cliCommand, "")
	return out, err
}

func historyPrompter(input string) *prompter {
	prompter := newPrompter(strings.NewReader(input), io.Discard)
	prompter.readPassword = func() ([]byte, error) { return []byte("synthetic-password"), nil }
	return prompter
}

func setHistoryNote(t *testing.T, item *model.Item, value string) {
	t.Helper()
	note, index, err := item.Note()
	require.NoError(t, err)
	note.Content.Value = value
	require.NoError(t, item.SetNote(index, note))
}
