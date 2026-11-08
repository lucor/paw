// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestUnlockVaultWrapsDetailedStorageErrors(t *testing.T) {
	test.NewTempApp(t)
	view := NewUnlockVault(model.VaultInfo{
		ID:   "AAAAAAAAAAAAAAAAAAAAAA",
		Name: "Personal",
	}, nil, nil, nil, nil, nil)
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(360, 640))
	window.Show()
	message := "could not migrate vault item storage: verify legacy Item: Item version StateHash mismatch"
	view.showError(errors.New(message))
	require.Equal(t, message, view.message.Text)
	require.Equal(t, fyne.TextWrapWord, view.message.Wrapping)
	require.Less(t, view.message.Size().Width, window.Canvas().Size().Width)
	require.Greater(t, view.message.MinSize().Height, view.password.MinSize().Height)
}

func TestUnlockVaultViewReportsPasswordErrorAndAllowsRetry(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Unlock")
	t.Cleanup(window.Close)

	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, nil, nil, nil, func() {}, func() {})
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()

	var password *widget.Entry
	var unlock *widget.Button
	var chooseAnother *widget.Button
	foundHeading := false
	foundPasswordLabel := false
	for _, object := range test.LaidOutObjects(view) {
		switch control := object.(type) {
		case *widget.Entry:
			if control.Password {
				password = control
			}
		case *widget.Button:
			if control.Text == "Unlock" {
				unlock = control
			}
			if control.Text == "Choose another vault" {
				chooseAnother = control
			}
		case *widget.Label:
			if control.Text == "Master password" {
				foundPasswordLabel = true
			}
			if control.Text == "Unlock Personal" {
				foundHeading = true
				require.Equal(t, fyne.TextAlignCenter, control.Alignment)
			}
		}
	}
	require.NotNil(t, password)
	require.True(t, foundHeading)
	require.True(t, foundPasswordLabel)
	require.Empty(t, password.PlaceHolder)
	require.NotNil(t, password.ActionItem)
	require.NotNil(t, unlock)
	require.True(t, unlock.Disabled())

	password.SetText("wrong password")
	view.showIncorrectPassword()
	require.Equal(t, "wrong password", password.Text, "preserve the entered password after an error")
	require.False(t, view.activity.Visible(), "hide the activity indicator after an error")
	require.Nil(t, window.Canvas().Overlays().Top(), "unlock feedback stays in the view")
	require.Equal(t, incorrectMasterPasswordMessage, view.passwordError.Text)
	require.True(t, view.passwordError.Visible())
	require.Equal(t, widget.DangerImportance, view.passwordError.Importance)
	require.True(t, password.Password, "the password remains masked")
	require.EqualError(t, password.Validate(), incorrectMasterPasswordMessage, "the Entry keeps its native validation state")
	require.False(t, unlock.Disabled(), "retry remains enabled with the preserved password")
	require.NotNil(t, chooseAnother)
	require.False(t, chooseAnother.Disabled(), "Choose another vault remains available")
	test.LaidOutObjects(window.Content())
	passwordTop := app.Driver().AbsolutePositionForObject(password).Y
	errorTop := app.Driver().AbsolutePositionForObject(view.passwordError).Y
	unlockTop := app.Driver().AbsolutePositionForObject(unlock).Y
	require.GreaterOrEqual(t, errorTop, passwordTop+password.Size().Height)
	require.Greater(t, unlockTop, errorTop)
	require.Same(t, password, window.Canvas().Focused(), "focus returns to the password field")

	password.SetText("correct password")
	require.False(t, view.passwordError.Visible(), "editing the password clears the error")
	require.NoError(t, password.Validate())
}

func TestUnlockVaultChooseAnotherClearsPassword(t *testing.T) {
	test.NewTempApp(t)
	chosen := false
	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, nil, nil, nil, nil, func() { chosen = true })
	window := test.NewTempWindow(t, view)
	window.Show()
	view.password.SetText("previous vault password")

	var chooseAnother *widget.Button
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
			break
		}
	}
	require.NotNil(t, chooseAnother)
	test.Tap(chooseAnother)
	require.True(t, chosen)
	require.Empty(t, view.password.Text, "leaving for the vault selector clears the previous password")
}

func TestUnlockVaultUnrelatedFailureDoesNotShowIncorrectPassword(t *testing.T) {
	test.NewTempApp(t)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, store.Close())
	view := NewUnlockVault(model.VaultInfo{ID: "AAAAAAAAAAAAAAAAAAAAAA", Name: "Personal"}, store, nil, nil, nil, nil)
	window := test.NewTempWindow(t, view)
	window.Show()
	view.password.SetText("master password")
	test.Tap(view.button)
	waitForUnlock(t, view)
	require.True(t, view.message.Visible())
	require.NotEmpty(t, view.message.Text)
	require.False(t, view.passwordError.Visible())
	require.Empty(t, view.password.Text, "unrelated failures keep the existing clear-on-error behavior")
}

func waitForUnlock(t *testing.T, view *UnlockVault) {
	t.Helper()
	select {
	case <-view.unlockDone:
	case <-time.After(15 * time.Second):
		t.Fatal("unlock UI update did not complete")
	}
}

func TestUnlockVaultContentStaysAboveViewportCenter(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(440, 800))
	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, nil, nil, func() {}, nil, nil)
	window.SetContent(view)
	window.Show()

	var unlock *widget.Button
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Unlock" {
			unlock = button
			break
		}
	}
	require.NotNil(t, unlock)

	position := fyne.CurrentApp().Driver().AbsolutePositionForObject(unlock)
	require.Less(t, position.Y+unlock.Size().Height/2, window.Canvas().Size().Height/2,
		"the unlock form should stay in the upper portion of a tall window")
}

func unlockViewText(t *testing.T, view fyne.CanvasObject) string {
	t.Helper()
	var text []string
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text != "" {
			text = append(text, label.Text)
		}
	}
	return strings.Join(text, "\n")
}
