// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestEditScriptedLoginFieldsWithoutTTY(t *testing.T) {
	base, vault := addTestCommand(t)
	base.prompter = newPrompter(strings.NewReader(""), base.errOut)
	item, err := model.NewLogin("GitHub", "old-user", "old-password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, base.storage.StoreItem(vault, item))
	path := "Personal/login/GitHub"
	out, stderr, err := executeSubcommand(t, []string{"edit", "--session", "active", "--name", "GitHub New", "--username", "mario", "--password-stdin", path}, newEditCommand, base, "new-password\n")
	require.NoError(t, err)
	require.Empty(t, out)
	require.Empty(t, stderr)
	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	updated, err := loaded.FindItem("GitHub New", model.LoginItemType)
	require.NoError(t, err)
	got, err := base.storage.LoadItem(loaded, updated.ID)
	require.NoError(t, err)
	login, _, err := got.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, "mario", login.Username.Value)
	require.Equal(t, "new-password", login.Password.Value)
}

func TestEditPasswordFromStdinDoesNotSavePartialChanges(t *testing.T) {
	base, vault := addTestCommand(t)
	item, err := model.NewLogin("GitHub", "old-user", "old-password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, base.storage.StoreItem(vault, item))
	_, _, err = executeSubcommand(t, []string{"edit", "--session", "active", "--name", "Renamed", "--username", "mario", "--password-stdin", "Personal/login/GitHub"}, newEditCommand, base, "\n")
	require.Error(t, err)
	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	unchanged, err := loaded.FindItem("GitHub", model.LoginItemType)
	require.NoError(t, err)
	got, err := base.storage.LoadItem(loaded, unchanged.ID)
	require.NoError(t, err)
	login, _, err := got.BasicAuth()
	require.NoError(t, err)
	require.Equal(t, "old-user", login.Username.Value)
	require.Equal(t, "old-password", login.Password.Value)
}
