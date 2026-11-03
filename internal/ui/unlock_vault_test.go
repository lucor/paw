// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

func TestUnlockVaultViewReportsPasswordErrorAndOpensVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Unlock")
	t.Cleanup(window.Close)

	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	store := &storage.StorageMock{}
	wrongPasswordStarted := make(chan struct{})
	releaseWrongPassword := make(chan struct{})
	store.OnLoadVaultKey = func(id, password string) (*paw.Key, error) {
		if id != "personal" {
			t.Errorf("unexpected vault ID %q", id)
		}
		if password == "wrong password" {
			close(wrongPasswordStarted)
			<-releaseWrongPassword
			return nil, &age.NoIdentityMatchError{}
		}
		if password != "correct password" {
			t.Errorf("unexpected password received by storage")
		}
		return key, nil
	}
	store.OnLoadVault = func(id string, loadedKey *paw.Key) (*model.Vault, error) {
		if id != "personal" || loadedKey != key {
			t.Errorf("unexpected vault load request")
		}
		return vault, nil
	}
	opened := make(chan *model.Vault, 1)
	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, store, func(openedVault *model.Vault) {
		opened <- openedVault
	}, nil, func() {}, func() {})
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
	test.Tap(unlock)
	require.Equal(t, "wrong password", password.Text, "preserve the entered password after submitting")
	select {
	case <-wrongPasswordStarted:
	case <-time.After(time.Second):
		t.Fatal("vault key loading did not start")
	}
	require.True(t, view.activity.Visible(), "show the activity indicator while unlocking")
	close(releaseWrongPassword)
	waitForUnlock(t, view)
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
	test.Tap(unlock)
	select {
	case got := <-opened:
		require.Same(t, vault, got)
	case <-time.After(time.Second):
		t.Fatal("vault was not opened")
	}
	waitForUnlock(t, view)
	require.Empty(t, password.Text, "leaving Unlock clears the password")
}

func TestUnlockVaultChooseAnotherClearsPassword(t *testing.T) {
	test.NewTempApp(t)
	chosen := false
	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, &storage.StorageMock{}, nil, nil, nil, func() { chosen = true })
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
	store := &storage.StorageMock{}
	store.OnLoadVaultKey = func(string, string) (*paw.Key, error) { return nil, errors.New("vault I/O failed") }
	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, store, nil, nil, nil, nil)
	window := test.NewTempWindow(t, view)
	window.Show()
	view.password.SetText("master password")
	test.Tap(view.button)
	waitForUnlock(t, view)
	require.Contains(t, unlockViewText(t, view), "vault I/O failed")
	require.False(t, view.passwordError.Visible())
	require.Empty(t, view.password.Text, "unrelated failures keep the existing clear-on-error behavior")
}

func waitForUnlock(t *testing.T, view *UnlockVault) {
	t.Helper()
	select {
	case <-view.unlockDone:
	case <-time.After(time.Second):
		t.Fatal("unlock UI update did not complete")
	}
}

func TestUnlockVaultContentStaysAboveViewportCenter(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(440, 800))
	view := NewUnlockVault(model.VaultInfo{ID: "personal", Name: "Personal"}, &storage.StorageMock{}, nil, func() {}, nil, nil)
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
