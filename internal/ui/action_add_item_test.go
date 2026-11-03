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

// TestAddPasswordItemOpensGenerator checks a new password item uses application settings.
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
		state: &model.AppState{Settings: model.NewDefaultSettings()},
		vault: vault,
	}
	application.addItem(model.PasswordItemType)

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
		state: &model.AppState{Settings: model.NewDefaultSettings()},
		vault: vault,
	}
	application.addItem(model.SSHKeyItemType)

	labels := make(map[string]bool)
	for _, object := range test.LaidOutObjects(window.Canvas().Content()) {
		if button, ok := object.(*widget.Button); ok {
			labels[button.Text] = true
		}
	}
	require.True(t, labels["Generate"])
	require.True(t, labels["Import"])
}

func TestLoginEditorSettingsActionOpensTOTPDialog(t *testing.T) {
	for _, isNew := range []bool{true, false} {
		t.Run(map[bool]string{true: "new", false: "edit"}[isNew], func(t *testing.T) {
			appDriver := test.NewApp()
			t.Cleanup(appDriver.Quit)
			window := appDriver.NewWindow("Paw")
			t.Cleanup(window.Close)
			item := loginEditorFixture(t)
			if isNew {
				var err error
				item, err = model.NewLogin("New login", "", "", model.PasswordSettings{Mode: paw.CustomPassword})
				require.NoError(t, err)
			}
			application := &app{
				win:   window,
				state: &model.AppState{Settings: model.NewDefaultSettings()},
			}
			view := application.makeLoginEditView(item, isNew)
			window.SetContent(view)
			settings := findPilotIconButton(t, view, "Authenticator settings")
			test.Tap(settings)
			popup := totpDialogOverlay(window)
			require.NotNil(t, popup)
			require.Contains(t, totpDialogTexts(popup), totpSettingsDialogTitle)
		})
	}
}
