// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"strings"
	"testing"

	"charm.land/huh/v2"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
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

func TestEditCustomFieldCancellationDoesNotSavePartialChanges(t *testing.T) {
	base, vault := addTestCommand(t)
	item, err := model.NewNote("Memo", "content")
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{
		{FieldType: cxf.FieldTypeString, Label: "Existing", Value: "keep"},
	}}))
	require.NoError(t, base.storage.StoreItem(vault, item))

	prompts := testInteractivePrompter(t, "2\n1\nNew field\n")
	formCount := 0
	prompts.runForm = func(form *huh.Form) error {
		formCount++
		if formCount == 4 {
			return errCanceled
		}
		return form.WithAccessible(true).Run()
	}
	base.prompter = prompts
	_, _, err = executeSubcommand(t, []string{"edit", "--session", "active", "Personal/note/Memo"}, newEditCommand, base, "")
	require.ErrorIs(t, err, errCanceled)

	loaded, err := base.storage.LoadVault(vault.Accounts[0].ID, vault.Key())
	require.NoError(t, err)
	summary, err := loaded.FindItem("Memo", model.NoteItemType)
	require.NoError(t, err)
	updated, err := base.storage.LoadItem(loaded, summary.ID)
	require.NoError(t, err)
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "Existing", groups[0].Credential.Fields[0].Label)
	require.Equal(t, "keep", groups[0].Credential.Fields[0].Value)
	require.Len(t, groups[0].Credential.Fields, 1)
}
