// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	test "go.lucor.dev/paw/internal/test"
)

func TestFailedCloneDeletesVaultAndRestoresView(t *testing.T) {
	test.NewTempApp(t)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	vault := test.CreateVault(t, store, "Clone")
	id := vault.Accounts[0].ID
	window := test.NewTempWindow(t, nil)
	previous := widget.NewLabel("Previous")
	a := &app{
		storage:       store,
		win:           window,
		main:          container.NewVScroll(widget.NewLabel("Clone")),
		vault:         vault,
		vaultID:       id,
		unlockedVault: map[string]*model.Vault{id: vault},
	}
	run := &syncConnectRun{
		cloneCreatedVault:   vault,
		previousMainContent: previous,
	}
	require.NoError(t, a.cleanupFailedClone(run, nil))
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
	var current fyne.CanvasObject
	fyne.DoAndWait(func() { current = a.main.Content })
	require.Same(t, previous, current)
	require.Nil(t, a.vault)
	require.Empty(t, a.unlockedVault)
}
