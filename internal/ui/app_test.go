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
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestFirstRunUsesAddVaultAndReturnsFromCreateAndRestore(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) { return nil, nil }
	state := &paw.AppState{Settings: paw.NewDefaultSettings()}
	state.Settings.StartupVault = paw.StartupVaultLastUsed
	state.Settings.LastVaultID = "previously-used"
	application := &app{win: window, storage: store, state: state}
	application.main = application.makeApp()
	window.SetContent(application.main)
	window.Show()

	objects := test.LaidOutObjects(application.main)
	texts := unlockViewText(t, application.main)
	for _, expected := range []string{"Add vault", "Create new vault", "Restore from backup"} {
		require.Contains(t, texts, expected)
	}
	for _, obsolete := range []string{"Welcome to Paw", "Connect via Sync"} {
		require.NotContains(t, texts, obsolete)
	}
	for _, object := range objects {
		if button, ok := object.(*widget.Button); ok {
			require.NotEqual(t, icons.Back, button.Icon, "first-run Add vault has no Back button")
		}
	}

	test.Tap(findNavigationRowButton(t, application.main, "Create new vault"))
	var back *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			back = button
			break
		}
	}
	require.NotNil(t, back)
	test.Tap(back)
	require.Same(t, application.main, window.Content(), "Cancel from Create Vault returns to first-run Add vault")
	require.Contains(t, unlockViewText(t, window.Content()), "Add vault")

	test.Tap(findNavigationRowButton(t, application.main, "Restore from backup"))
	var restoreBack *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			restoreBack = button
			break
		}
	}
	require.NotNil(t, restoreBack)
	require.Contains(t, unlockViewText(t, window.Content()), "Restore from backup")
	test.Tap(restoreBack)
	require.Same(t, application.main, window.Content(), "Cancel from Restore returns to first-run Add vault")
	require.Contains(t, unlockViewText(t, window.Content()), "Add vault")
}

func TestMakeAppUsesLastUsedVaultSetting(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	vaults := []storage.VaultInfo{{ID: "first", Name: "First"}, {ID: "last", Name: "Last"}}
	store.OnVaults = func() ([]storage.VaultInfo, error) { return vaults, nil }
	state := &paw.AppState{Settings: paw.NewDefaultSettings()}
	state.Settings.LastVaultID = "last"
	application := &app{win: window, storage: store, state: state}
	content := application.makeApp()

	var unlock *UnlockVault
	for _, object := range test.LaidOutObjects(content) {
		if candidate, ok := object.(*UnlockVault); ok {
			unlock = candidate
		}
	}
	require.NotNil(t, unlock, "last used vault should open to its unlock screen")
	require.Equal(t, "last", unlock.info.ID)

	application.main = content
	window.SetContent(application.main)
	window.Show()
	loadKeyCalls, loadVaultCalls := 0, 0
	store.OnLoadVaultKey = func(string, string) (*paw.Key, error) {
		loadKeyCalls++
		return nil, nil
	}
	store.OnLoadVault = func(string, *paw.Key) (*model.Vault, error) {
		loadVaultCalls++
		return nil, nil
	}
	var chooseAnother *widget.Button
	for _, object := range test.LaidOutObjects(unlock) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
			break
		}
	}
	require.NotNil(t, chooseAnother, "last-used startup unlock should offer the local vault selector")
	test.Tap(chooseAnother)
	require.Equal(t, "last", state.Settings.LastVaultID)
	require.Nil(t, application.vault)
	require.Empty(t, application.unlockedVault)
	require.Zero(t, loadKeyCalls)
	require.Zero(t, loadVaultCalls)

	var selector *widget.List
	for _, object := range test.LaidOutObjects(application.main) {
		if list, ok := object.(*widget.List); ok {
			selector = list
			break
		}
	}
	require.NotNil(t, selector)
	selector.Select(0)
	unlock, unlockScreen := application.main.Content.(*UnlockVault)
	require.True(t, unlockScreen)
	chooseAnother = nil
	for _, object := range test.LaidOutObjects(unlock) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
		}
	}
	require.NotNil(t, chooseAnother)
	test.Tap(chooseAnother)
	selectorCount := 0
	for _, object := range test.LaidOutObjects(application.main) {
		if _, ok := object.(*widget.List); ok {
			selectorCount++
		}
	}
	require.Equal(t, 1, selectorCount, "repeated selection should replace, not stack, the selector")
	require.Equal(t, "last", state.Settings.LastVaultID)
	require.Zero(t, loadKeyCalls)
	require.Zero(t, loadVaultCalls)

	state.Settings.LastVaultID = "missing"
	content = application.makeApp()
	var selectorFound bool
	for _, object := range test.LaidOutObjects(content) {
		if _, ok := object.(*widget.List); ok {
			selectorFound = true
		}
	}
	require.True(t, selectorFound, "missing last used vault should fall back to selector")
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
