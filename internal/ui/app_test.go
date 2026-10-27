// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"testing"

	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

// TestWelcomeNavigation checks the welcome actions open their screens and return.
func TestWelcomeNavigation(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) { return nil, nil }
	a := &app{win: window, storage: store}
	a.main = a.makeApp()
	window.SetContent(a.main)
	window.Show()

	var buttons []*widget.Button
	for _, object := range test.LaidOutObjects(a.main) {
		if button, ok := object.(*widget.Button); ok {
			buttons = append(buttons, button)
		}
	}
	require.Len(t, buttons, 3)
	for i, title := range []string{"Create new vault", "Restore from backup", "Connect via Sync"} {
		test.Tap(buttons[i])
		var back *widget.Button
		foundTitle := false
		foundCreateButton := false
		for _, object := range test.LaidOutObjects(window.Content()) {
			switch object := object.(type) {
			case *widget.Label:
				foundTitle = foundTitle || object.Text == title
			case *widget.Button:
				if object.Text == "Back" || object.Text == "" && object.Icon != nil {
					back = object
				}
				foundCreateButton = foundCreateButton || object.Text == "Create vault"
			}
		}
		require.True(t, foundTitle, "placeholder must show %q", title)
		if i == 0 {
			require.True(t, foundCreateButton)
		}
		require.NotNil(t, back)
		test.Tap(back)
		require.Same(t, a.main, window.Content())
	}
}

// TestCreateVaultCallsStorage checks the form passes the name and password to storage.
func TestCreateVaultCallsStorage(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	var gotName, gotPassword string
	store.OnCreateVault = func(name, password string) (*model.Vault, error) {
		gotName, gotPassword = name, password
		return nil, errors.New("storage failure")
	}
	a := &app{win: window, main: container.NewVScroll(widget.NewLabel("Vaults")), storage: store}
	a.showCreateVaultView()
	window.Show()

	var entries []*widget.Entry
	var create *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		switch object := object.(type) {
		case *widget.Entry:
			entries = append(entries, object)
		case *widget.Button:
			if object.Text == "Create vault" {
				create = object
			}
		}
	}
	require.Len(t, entries, 3)
	require.NotNil(t, create)
	entries[0].SetText("Personal")
	entries[1].SetText("secret password")
	entries[2].SetText("secret password")
	test.Tap(create)
	require.Equal(t, "Personal", gotName)
	require.Equal(t, "secret password", gotPassword)
	foundError := false
	for _, object := range test.LaidOutObjects(window.Content()) {
		if label, ok := object.(*widget.Label); ok && label.Text == "storage failure" {
			foundError = true
		}
	}
	require.True(t, foundError)
}
