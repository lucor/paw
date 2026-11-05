// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

func TestDeleteItemUsesSharedDestructiveConfirmation(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	item, err := model.NewPassword("GitHub", "secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)

	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.UpdateVaultIdentity(vault, "Personal", model.DefaultVaultAppearance()))
	require.NoError(t, store.StoreItem(vault, item))
	a := &app{win: window, vault: vault, storage: store}

	a.deleteItem(item)
	objects := test.LaidOutObjects(window.Canvas().Overlays().Top())
	labels := make([]string, 0)
	buttons := make(map[string]*widget.Button)
	for _, object := range objects {
		if label, ok := object.(*widget.Label); ok {
			labels = append(labels, label.Text)
		}
		if button, ok := object.(*widget.Button); ok {
			buttons[button.Text] = button
		}
	}
	require.Contains(t, labels, "Delete item?")
	require.Contains(t, labels, "Permanently delete \"GitHub\" from this vault? This action cannot be undone.")
	require.Contains(t, buttons, "Cancel")
	require.Contains(t, buttons, "Delete item")
	require.Nil(t, buttons["Cancel"].Icon)
	require.Nil(t, buttons["Delete item"].Icon)
	require.Equal(t, widget.DangerImportance, buttons["Delete item"].Importance)
	_, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err, "opening the dialog must not delete the item")

	test.Tap(buttons["Cancel"])
	_, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err, "cancel must preserve the item")
	require.NoError(t, store.Close())

	a.deleteItem(item)
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Delete item" {
			test.Tap(button)
			break
		}
	}
	require.Len(t, vault.Accounts[0].Items, 1, "failed deletion must preserve the item summary")
	require.NotNil(t, window.Canvas().Overlays().Top(), "the storage error should be reported")
}
