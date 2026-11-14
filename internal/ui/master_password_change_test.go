// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2/container"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	test "go.lucor.dev/paw/internal/test"
)

// TestMasterPasswordChangeLocksVault exercises a real password rekey through FileStorage.
func TestMasterPasswordChangeLocksVault(t *testing.T) {
	test.NewTempApp(t)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	vault := test.CreateVaultWithPassword(t, store, "Personal", "old password")
	id := vault.Accounts[0].ID
	w := test.NewTempWindow(t, nil)
	a := &app{win: w, storage: store, main: container.NewVScroll(nil), vault: vault, vaultID: id, settingsVaultID: id, unlockedVault: map[string]*model.Vault{id: vault}}
	a.showChangeMasterPassword()
	v := a.passwordChange
	require.NotNil(t, v)
	v.current.SetText("incorrect password")
	v.password.Entry.SetText("new password")
	v.confirm.SetText("new password")
	test.Tap(v.submit)
	select {
	case <-v.operationDone:
	case <-time.After(time.Minute):
		t.Fatal("password change did not finish")
	}
	require.True(t, v.currentError.Visible())
	require.False(t, v.progress.Visible())
	require.Same(t, vault, a.vault)
	v.current.SetText("old password")
	require.False(t, v.currentError.Visible())
	test.Tap(v.submit)
	select {
	case <-v.operationDone:
	case <-time.After(time.Minute):
		t.Fatal("password change did not finish")
	}
	require.Nil(t, a.vault)
	require.Empty(t, a.unlockedVault)
	require.Empty(t, v.current.Text)
	require.Empty(t, v.password.Entry.Text)
	_, err = store.LoadVaultKey(id, "old password")
	require.Error(t, err)
	_, err = store.LoadVaultKey(id, "new password")
	require.NoError(t, err)
}
