// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestCreateVaultEndToEnd(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	a := &app{
		win: window, storage: store,
		state:         &model.AppState{Settings: model.NewDefaultSettings()},
		filter:        make(map[string]*vaultFilterOptions),
		unlockedVault: make(map[string]*model.Vault),
	}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()
	test.Tap(findNavigationRowButton(t, window.Content(), "Create new vault"))
	var entries []*widget.Entry
	var submit *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		switch control := object.(type) {
		case *widget.Entry:
			entries = append(entries, control)
		case *widget.Button:
			if control.Text == "Create vault" {
				submit = control
			}
		}
	}
	require.Len(t, entries, 3)
	require.NotNil(t, submit)
	entries[0].SetText("Personal")
	entries[1].SetText("example master password")
	entries[2].SetText("example master password")
	require.False(t, submit.Disabled())
	test.Tap(submit)
	require.NotNil(t, a.vault)
	id := a.vault.Accounts[0].ID
	infos, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, infos, 1)
	require.Equal(t, id, infos[0].ID)
	require.Equal(t, "Personal", infos[0].Name)
	reopened, err := storage.NewFileStorage(store.Root())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	a.storage = reopened
	a.lockAllVaults()
	require.Nil(t, a.vault)
	unlock, ok := a.main.Content.(*UnlockVault)
	require.True(t, ok)
	unlock.password.SetText("wrong password")
	test.Tap(unlock.button)
	waitForUnlock(t, unlock)
	require.True(t, unlock.passwordError.Visible())
	require.Equal(t, incorrectMasterPasswordMessage, unlock.passwordError.Text)
	require.Equal(t, "wrong password", unlock.password.Text)
	require.False(t, unlock.activity.Visible())
	unlock.password.SetText("example master password")
	test.Tap(unlock.button)
	waitForUnlock(t, unlock)
	require.NotNil(t, a.vault)
	require.Equal(t, id, a.vault.Accounts[0].ID)
	require.Equal(t, "Personal", a.vault.Metadata.VaultName)
	require.Empty(t, unlock.password.Text)
}
