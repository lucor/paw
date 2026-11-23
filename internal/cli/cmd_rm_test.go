// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"io"
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestRemoveWithYesNeedsNoTerminalAndProducesNoOutput(t *testing.T) {
	base, vault := addTestCommand(t)
	item, err := model.NewNote("Memo", "content")
	require.NoError(t, err)
	require.NoError(t, base.storage.StoreItem(vault, item))
	base.prompter = newPrompter(strings.NewReader("ignored input"), io.Discard)
	base.prompter.readPassword = func() ([]byte, error) { t.Fatal("unexpected password prompt"); return nil, nil }
	out, stderr, err := executeSubcommand(t, []string{"rm", "--session", "active", "--yes", "Personal/note/Memo"}, newRemoveCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, stderr)
	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	_, err = loaded.FindItem("Memo", model.NoteItemType)
	require.Error(t, err)
}

func TestRemoveConfirmationAndCancellation(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		answer    string
		cancel    bool
		remaining bool
	}{
		{name: "confirmed", answer: "y\n"},
		{name: "declined", answer: "n\n", remaining: true},
		{name: "cancelled", cancel: true, remaining: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			base, vault := addTestCommand(t)
			item, err := model.NewNote("Memo", "content")
			require.NoError(t, err)
			require.NoError(t, base.storage.StoreItem(vault, item))
			base.prompter = testInteractivePrompter(t, testCase.answer)
			if testCase.cancel {
				base.prompter.runForm = func(*huh.Form) error { return errCanceled }
			}
			out, stderr, err := executeSubcommand(t, []string{"rm", "--session", "active", "Personal/note/Memo"}, newRemoveCommand, base, "")
			if testCase.cancel {
				require.ErrorIs(t, err, errCanceled)
			} else {
				require.NoError(t, err)
			}
			require.Empty(t, out)
			require.Empty(t, stderr)
			loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
			require.NoError(t, err)
			_, err = loaded.FindItem("Memo", model.NoteItemType)
			if testCase.remaining {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
		})
	}
}

func TestRemoveWithoutYesRequiresTTYAndKeepsItem(t *testing.T) {
	base, vault := addTestCommand(t)
	item, err := model.NewNote("Memo", "content")
	require.NoError(t, err)
	require.NoError(t, base.storage.StoreItem(vault, item))
	base.prompter = newPrompter(strings.NewReader("y\n"), io.Discard)
	_, _, err = executeSubcommand(t, []string{"rm", "--session", "active", "Personal/note/Memo"}, newRemoveCommand, base, "")
	require.ErrorContains(t, err, "requires stdin to be a terminal")
	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	_, err = loaded.FindItem("Memo", model.NoteItemType)
	require.NoError(t, err)
}

func TestRemoveMissingItemReturnsError(t *testing.T) {
	base, _ := addTestCommand(t)
	_, _, err := executeSubcommand(t, []string{"rm", "--session", "active", "--yes", "Personal/note/Missing"}, newRemoveCommand, base, "")
	require.Error(t, err)
}
