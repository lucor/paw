// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
)

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
