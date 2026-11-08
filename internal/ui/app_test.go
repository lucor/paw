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
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	state.Settings.StartupVault = model.StartupVaultLastUsed
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

func TestMakeAppAndAutoLockUseLastUsedVaultSetting(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	first, err := model.NewVault(key, "First")
	require.NoError(t, err)
	last, err := model.NewVault(key, "Last")
	require.NoError(t, err)
	for _, vault := range []*model.Vault{first, last} {
		require.NoError(t, store.UpdateVaultIdentity(vault, vault.Metadata.VaultName, model.DefaultVaultAppearance()))
	}
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	state.Settings.LastVaultID = last.Accounts[0].ID
	application := &app{win: window, storage: store, state: state}
	content := application.makeApp()

	var unlock *UnlockVault
	for _, object := range test.LaidOutObjects(content) {
		if candidate, ok := object.(*UnlockVault); ok {
			unlock = candidate
		}
	}
	require.NotNil(t, unlock, "last used vault should open to its unlock screen")
	require.Equal(t, last.Accounts[0].ID, unlock.info.ID)

	application.main = content
	window.SetContent(application.main)
	window.Show()
	var chooseAnother *widget.Button
	for _, object := range test.LaidOutObjects(unlock) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
			break
		}
	}
	require.NotNil(t, chooseAnother, "last-used startup unlock should offer the local vault selector")

	application.vault = last
	application.vaultID = last.Accounts[0].ID
	application.unlockedVault = map[string]*model.Vault{last.Accounts[0].ID: last}
	application.lockAllVaults()
	unlock, unlockScreen := application.main.Content.(*UnlockVault)
	require.True(t, unlockScreen, "auto-lock should preserve the last-used unlock screen")
	require.Equal(t, last.Accounts[0].ID, unlock.info.ID)
	chooseAnother = nil
	for _, object := range test.LaidOutObjects(unlock) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
			chooseAnother = button
			break
		}
	}
	require.NotNil(t, chooseAnother, "auto-lock should offer the selector when multiple vaults exist")
	test.Tap(chooseAnother)
	require.Equal(t, last.Accounts[0].ID, state.Settings.LastVaultID)
	require.Nil(t, application.vault)
	require.Empty(t, application.unlockedVault)

	var selector *widget.List
	for _, object := range test.LaidOutObjects(application.main) {
		if list, ok := object.(*widget.List); ok {
			selector = list
			break
		}
	}
	require.NotNil(t, selector)
	selector.Select(0)
	unlock, unlockScreen = application.main.Content.(*UnlockVault)
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
	require.Equal(t, last.Accounts[0].ID, state.Settings.LastVaultID)

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

func TestManualLockShowsVaultSelectorOnlyWhenMultipleVaultsExist(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		vaultCount int
	}{
		{name: "single vault", vaultCount: 1},
		{name: "multiple vaults", vaultCount: 2},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			driver := test.NewApp()
			t.Cleanup(driver.Quit)
			window := driver.NewWindow("Paw")
			t.Cleanup(window.Close)
			store, err := storage.NewFileStorage(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			key, err := paw.MakeOneTimeKey()
			require.NoError(t, err)
			vaults := make([]*model.Vault, 0, testCase.vaultCount)
			for index := 0; index < testCase.vaultCount; index++ {
				name := []string{"Personal", "Work"}[index]
				vault, err := model.NewVault(key, name)
				require.NoError(t, err)
				require.NoError(t, store.UpdateVaultIdentity(vault, name, model.DefaultVaultAppearance()))
				vaults = append(vaults, vault)
			}
			available, err := store.Vaults()
			require.NoError(t, err)

			current := vaults[0]
			currentID := current.Accounts[0].ID
			application := &app{
				win:           window,
				main:          container.NewVScroll(widget.NewLabel("Unlocked vault")),
				storage:       store,
				vault:         current,
				vaultID:       currentID,
				unlockedVault: map[string]*model.Vault{currentID: current},
			}
			window.SetContent(application.main)
			lock := application.makeVaultActions()[4]
			lock.Action()

			unlock, ok := application.main.Content.(*UnlockVault)
			require.True(t, ok, "manual lock should keep the just-locked vault's unlock screen")
			require.Equal(t, currentID, unlock.info.ID)
			var chooseAnother *widget.Button
			for _, object := range test.LaidOutObjects(unlock) {
				if button, ok := object.(*widget.Button); ok && button.Text == "Choose another vault" {
					chooseAnother = button
					break
				}
			}
			require.Equal(t, testCase.vaultCount > 1, chooseAnother != nil)
			if chooseAnother == nil {
				return
			}

			test.Tap(chooseAnother)
			var selector *widget.List
			for _, object := range test.LaidOutObjects(application.main) {
				if list, ok := object.(*widget.List); ok {
					selector = list
					break
				}
			}
			require.NotNil(t, selector)
			require.Equal(t, testCase.vaultCount, selector.Length())
			selector.Select(1)
			nextUnlock, ok := application.main.Content.(*UnlockVault)
			require.True(t, ok)
			require.Equal(t, available[1].ID, nextUnlock.info.ID)
		})
	}
}

func TestCreateVaultReportsSaveFailure(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	var gotName, gotPassword string
	view := NewCreateVault(func() {}, func(name, password string, _ model.VaultAppearance) error {
		gotName, gotPassword = name, password
		return errors.New("storage failure")
	})
	window.SetContent(view)
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
