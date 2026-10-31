// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestMasterPasswordChangeLocksVault(t *testing.T) {
	test.NewTempApp(t)
	store, err := storage.NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	vault, err := store.CreateVault("Personal", "old password")
	require.NoError(t, err)
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
	require.True(t, v.progress.Visible())
	require.True(t, v.activity.Visible())
	require.Eventually(t, func() bool {
		var visible bool
		fyne.DoAndWait(func() {
			visible = v.currentError.Visible()
		})
		return visible
	}, 15*time.Second, 10*time.Millisecond)
	require.False(t, v.progress.Visible())
	require.Same(t, vault, a.vault)
	v.current.SetText("old password")
	require.False(t, v.currentError.Visible())
	test.Tap(v.submit)
	require.Eventually(t, func() bool {
		var complete bool
		fyne.DoAndWait(func() {
			complete = a.passwordChange == nil
		})
		return complete
	}, 15*time.Second, 10*time.Millisecond)
	require.Nil(t, a.vault)
	require.Empty(t, a.unlockedVault)
	require.Empty(t, v.current.Text)
	require.Empty(t, v.password.Entry.Text)
	_, err = store.LoadVaultKey(id, "old password")
	require.Error(t, err)
	_, err = store.LoadVaultKey(id, "new password")
	require.NoError(t, err)
}

func TestMasterPasswordFormValidationAndClear(t *testing.T) {
	test.NewTempApp(t)
	v := newMasterPasswordView(func() {})
	w := test.NewTempWindow(t, v.view)
	w.Resize(fyne.NewSize(320, 540))
	require.True(t, v.submit.Disabled())
	v.current.SetText("old password")
	v.password.Entry.SetText("new password")
	v.confirm.SetText("mismatch")
	test.LaidOutObjects(v.view)
	require.NotEmpty(t, v.password.strength.label.Text)
	require.Greater(t, v.password.strength.label.Size().Width, float32(0))
	require.True(t, v.submit.Disabled())
	v.confirm.FocusLost()
	require.Error(t, v.confirm.Validate())
	v.confirm.SetText("new password")
	require.NoError(t, v.confirm.Validate())
	require.False(t, v.submit.Disabled())
	v.clear()
	require.Empty(t, v.current.Text)
	require.Empty(t, v.password.Entry.Text)
	require.Empty(t, v.confirm.Text)
	require.True(t, v.submit.Disabled())
}

func TestMasterPasswordActivityIsCenteredDuringOperation(t *testing.T) {
	test.NewTempApp(t)
	v := newMasterPasswordView(func() {})
	w := test.NewTempWindow(t, v.view)
	w.Resize(fyne.NewSize(400, 640))
	test.LaidOutObjects(v.view)
	require.False(t, v.progress.Visible())

	v.progress.Show()
	v.activity.Start()
	w.Canvas().Refresh(v.view)
	test.LaidOutObjects(v.view)

	require.True(t, v.activity.Visible())
	center := fyne.NewPos(v.progress.Size().Width/2, v.progress.Size().Height/2)
	activityCenter := v.activity.Position().Add(fyne.NewPos(v.activity.Size().Width/2, v.activity.Size().Height/2))
	require.InDelta(t, center.X, activityCenter.X, 1)
	require.InDelta(t, center.Y, activityCenter.Y, 1)
	v.activity.Stop()
}
