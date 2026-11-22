// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	test "go.lucor.dev/paw/internal/test"
)

func TestListEmptyAndFilteredResultsHaveNoOutput(t *testing.T) {
	store, err := storage.NewFileStorage(filepath.Join(t.TempDir(), "data"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault := test.CreateVaultWithPassword(t, store, "Personal", "password")
	prompter := newPrompter(strings.NewReader(""), io.Discard)
	prompter.readPassword = func() ([]byte, error) { return []byte("password"), nil }
	base := &command{storage: store, prompter: prompter}

	out, stderr, err := executeSubcommand(t, []string{"ls", "Personal/*"}, newListCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, stderr)

	item, err := model.NewNote("memo", "note")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	out, stderr, err = executeSubcommand(t, []string{"ls", "Personal/note/missing"}, newListCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, stderr)

	out, stderr, err = executeSubcommand(t, []string{"ls", "Personal/*"}, newListCommand, base, "")
	require.NoError(t, err)
	require.Empty(t, stderr)
	require.Equal(t, "Personal ("+shortVaultID(vault.Accounts[0].ID)+")\n└── note\n    └── memo\n", out)
}
