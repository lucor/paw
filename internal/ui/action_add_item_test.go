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

	var generatePassword fyne.Tappable
	for _, object := range test.LaidOutObjects(window.Canvas().Content()) {
		button, ok := object.(interface{ AccessibilityLabel() string })
		if ok && button.AccessibilityLabel() == "Generate password" {
			generatePassword, _ = object.(fyne.Tappable)
			break
		}
	}
	require.NotNil(t, generatePassword)
	test.Tap(generatePassword)
	require.NotNil(t, window.Canvas().Overlays().Top())
}

// TestAddSSHItemShowsCreationActions checks a new SSH item offers generate and import actions.
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
	require.True(t, labels["Generate"])
	require.True(t, labels["Import"])
}
