// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"

	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestAddPasswordItemOpensGenerator checks a new password item has generator preferences.
func TestAddPasswordItemOpensGenerator(t *testing.T) {
	appDriver := test.NewApp()
	t.Cleanup(appDriver.Quit)
	window := appDriver.NewWindow("Paw")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	application := &app{
		win:   window,
		main:  nil,
		state: &paw.AppState{Preferences: paw.NewDefaultPreferences()},
		vault: vault,
	}
	application.addItem(paw.PasswordItemType)

	var generatePassword *widget.Button
	for _, object := range test.LaidOutObjects(window.Canvas().Content()) {
		button, ok := object.(*widget.Button)
		if ok && button.Icon == icon.KeyOutlinedIconThemed {
			generatePassword = button
			break
		}
	}
	require.NotNil(t, generatePassword)
	require.NotPanics(t, func() { generatePassword.OnTapped() })
	require.NotNil(t, window.Canvas().Overlays().Top())
}

// TestAddSSHItemShowsCreationActions checks a new SSH item offers regenerate and import actions.
func TestAddSSHItemShowsCreationActions(t *testing.T) {
	appDriver := test.NewApp()
	t.Cleanup(appDriver.Quit)
	window := appDriver.NewWindow("Paw")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	application := &app{
		win:   window,
		state: &paw.AppState{Preferences: paw.NewDefaultPreferences()},
		vault: vault,
	}
	application.addItem(paw.SSHKeyItemType)

	labels := make(map[string]bool)
	for _, object := range test.LaidOutObjects(window.Canvas().Content()) {
		if button, ok := object.(*widget.Button); ok {
			labels[button.Text] = true
		}
	}
	require.True(t, labels["Regenerate key"])
	require.True(t, labels["Import key"])
}
