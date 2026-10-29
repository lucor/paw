// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
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
	view := NewUnlockVault(storage.VaultInfo{ID: "personal", Name: "Personal"}, store, func(openedVault *model.Vault) {
		opened <- openedVault
	}, nil, func() {})
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()

	var password *widget.Entry
	var unlock *widget.Button
	foundHeading := false
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
		case *widget.Label:
			require.NotEqual(t, "Master password", control.Text)
			if control.Text == "Unlock Personal" {
				foundHeading = true
				require.Equal(t, fyne.TextAlignCenter, control.Alignment)
			}
		}
	}
	require.NotNil(t, password)
	require.True(t, foundHeading)
	require.Empty(t, password.PlaceHolder)
	require.NotNil(t, password.ActionItem)
	require.NotNil(t, unlock)
	require.True(t, unlock.Disabled())

	password.SetText("wrong password")
	test.Tap(unlock)
	require.Empty(t, password.Text, "clear the password after submitting")
	select {
	case <-wrongPasswordStarted:
	case <-time.After(time.Second):
		t.Fatal("vault key loading did not start")
	}
	require.True(t, view.activity.Visible(), "show the activity indicator while unlocking")
	close(releaseWrongPassword)
	require.Eventually(t, func() bool {
		return strings.Contains(unlockViewText(t, view), "the password is incorrect")
	}, time.Second, 10*time.Millisecond)
	require.False(t, view.activity.Visible(), "hide the activity indicator after an error")
	require.Nil(t, window.Canvas().Overlays().Top(), "unlock feedback stays in the view")

	password.SetText("correct password")
	test.Tap(unlock)
	select {
	case got := <-opened:
		require.Same(t, vault, got)
	case <-time.After(time.Second):
		t.Fatal("vault was not opened")
	}
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
