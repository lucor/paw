// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/test"
)

type historyFailingWriter struct{ err error }

func (writer historyFailingWriter) Write([]byte) (int, error) { return 0, writer.err }

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
	path := storage.FormatVaultID(vault.Accounts[0].ID) + "/note/Current"

	listed, err := executeWithHistoryBase(t, base, []string{"history", path})
	require.NoError(t, err)
	require.Contains(t, listed, "INDEX")
	require.Contains(t, listed, "CAPTURED AT")
	require.NotContains(t, listed, "|")
	require.Less(t, strings.Index(listed, time.Unix(200, 0).Local().Format(humanTimeLayout)), strings.Index(listed, time.Unix(100, 0).Local().Format(humanTimeLayout)))
	require.NotContains(t, listed, "oldest-secret")
	require.NotContains(t, listed, "newest-secret")

	shown, err := executeWithHistoryBase(t, base, []string{"history", "show", path, "0"})
	require.NoError(t, err)
	require.Contains(t, shown, "History: "+time.Unix(200, 0).Local().Format(humanTimeLayout))
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
	path := storage.FormatVaultID(vault.Accounts[0].ID) + "/note/Current"

	base.prompter = historyPrompter(t, "n\n")
	canceled, err := executeWithHistoryBase(t, base, []string{"history", "restore", path, "0"})
	require.NoError(t, err)
	require.Empty(t, canceled)
	current, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", current.Title)
	base.prompter.runForm = func(*huh.Form) error { return errCanceled }
	_, err = executeWithHistoryBase(t, base, []string{"history", "restore", path, "0"})
	require.ErrorIs(t, err, errCanceled)
	current, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", current.Title)

	base.prompter = historyPrompter(t, "y\n")
	base.now = func() time.Time { return time.Unix(300, 0).UTC() }
	restored, diagnostics, err := executeSubcommand(t, []string{"history", "restore", path, "0"}, newHistoryCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, restored)
	require.Empty(t, diagnostics)
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
	path := storage.FormatVaultID(vault.Accounts[0].ID) + "/note/Current"
	listed, err := executeWithHistoryBase(t, base, []string{"history", path})
	require.NoError(t, err)
	require.Empty(t, listed)
	listed, err = executeWithHistoryBase(t, base, []string{"history", "list", path})
	require.NoError(t, err)
	require.Empty(t, listed)
	_, err = executeWithHistoryBase(t, base, []string{"history", "restore", path, "0"})
	require.ErrorContains(t, err, "no history")
}

func TestItemHistoryListPropagatesWriterError(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	history, err := item.History()
	require.NoError(t, err)
	old := *item
	old.Title = "Old"
	require.NoError(t, history.AddSnapshot(&old, 100))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	wantErr := errors.New("stdout unavailable")
	base.out = historyFailingWriter{err: wantErr}
	cmd := &historyCmd{command: base, itemPath: itemPath{
		vaultName: storage.FormatVaultID(vault.Accounts[0].ID),
		itemType:  model.NoteItemType,
		itemName:  "Current",
	}}
	require.ErrorIs(t, cmd.list(), wantErr)
}

func TestHistoryRestoreYesSkipsTTYAndRestoreNeedsTTY(t *testing.T) {
	store, vault, item, base := historyCLIFixture(t)
	history, err := item.History()
	require.NoError(t, err)
	old := *item
	old.Title = "Old"
	require.NoError(t, history.AddSnapshot(&old, 100))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	base.agentClient = func(string) (agent.PawAgent, error) {
		return addTestAgent{key: vault.Key()}, nil
	}
	base.prompter = newPrompter(strings.NewReader("yes\n"), io.Discard)
	path := storage.FormatVaultID(vault.Accounts[0].ID) + "/note/Current"
	_, _, err = executeSubcommand(t, []string{"history", "restore", "--session", "active", path, "0"}, newHistoryCommand, base, "")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	current, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", current.Title)
	_, _, err = executeSubcommand(t, []string{"history", "restore", "--session", "active", "--yes", path, "0"}, newHistoryCommand, base, "")
	require.NoError(t, err)
	vault, err = store.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	current, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Old", current.Title)
}

func historyCLIFixture(t *testing.T) (*storage.FileStorage, *model.Vault, *model.Item, *command) {
	t.Helper()
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault := test.CreateVaultWithPassword(t, store, "Personal", "synthetic-password")
	item, err := model.NewNote("Current", "current-secret")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	base := &command{
		storage:  store,
		prompter: historyPrompter(t, ""),
		now:      time.Now,
	}
	return store, vault, item, base
}

func executeWithHistoryBase(t *testing.T, base *command, args []string) (string, error) {
	t.Helper()
	out, _, err := executeSubcommand(t, args, newHistoryCommand, base, "")
	return out, err
}

func historyPrompter(t *testing.T, input string) *prompter {
	t.Helper()
	prompter := testInteractivePrompter(t, input)
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
