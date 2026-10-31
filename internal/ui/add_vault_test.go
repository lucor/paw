// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestAddVaultKeepsRestoreAction(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Add vault")
	t.Cleanup(window.Close)
	called := false
	view := NewAddVault(func() {}, func() {}, func() { called = true })
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 300))
	window.Show()

	var restoreTitle *widget.Label
	var restoreIcon bool
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == "Restore from backup" {
			restoreTitle = label
		}
		if icon, ok := object.(*widget.Icon); ok && icon.Resource == icons.ArchiveRestore {
			restoreIcon = true
		}
	}
	require.NotNil(t, restoreTitle)
	require.True(t, restoreIcon)
	test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(restoreTitle).Add(fyne.NewPos(restoreTitle.Size().Width/2, restoreTitle.Size().Height/2)))
	require.True(t, called)
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok {
			require.NotEqual(t, "Connect via Sync", label.Text)
		}
	}
}

func TestFirstRunAddVaultHidesBackButton(t *testing.T) {
	test.NewTempApp(t)
	view := NewAddVault(nil, func() {}, func() {})
	var titleFound bool
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == "Add vault" {
			titleFound = true
		}
		if button, ok := object.(*widget.Button); ok {
			require.NotEqual(t, icons.Back, button.Icon)
			require.NotEqual(t, "Connect via Sync", button.Text)
		}
	}
	require.True(t, titleFound)
}

func TestAddVaultDescriptionsWrapAtNarrowWidth(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Add vault")
	t.Cleanup(window.Close)
	view := NewAddVault(func() {}, func() {}, func() {})
	window.SetContent(view)
	window.Resize(fyne.NewSize(320, 540))
	window.Show()
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && (label.Text == "Start with an empty encrypted vault" || label.Text == "Create a vault from a backup file") {
			require.Equal(t, fyne.TextWrapWord, label.Wrapping)
			require.Equal(t, fyne.TextTruncateOff, label.Truncation)
			require.Greater(t, label.Size().Height, float32(0))
		}
	}
}
