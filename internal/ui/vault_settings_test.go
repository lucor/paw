// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestVaultSettingsViewOpensIdentityEditorAndReturns(t *testing.T) {
	test.NewTempApp(t)
	editCalled, backCalled := false, false
	view := NewVaultSettingsView("Personal", storage.DefaultVaultAppearance(), func() { editCalled = true }, func() {}, func() {}, func() { backCalled = true })
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(320, 540))
	row := findVaultSettingsRow(t, view)
	test.Tap(row)
	require.True(t, editCalled)
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	require.True(t, backCalled)
}

func TestVaultSettingsDeleteRowRequestsConfirmation(t *testing.T) {
	test.NewTempApp(t)
	deleteRequested := false
	view := NewVaultSettingsView("Personal", storage.DefaultVaultAppearance(), func() {}, func() {}, func() { deleteRequested = true }, func() {})
	test.Tap(findNavigationRowButton(t, view, "Delete vault"))
	require.True(t, deleteRequested)
}

func TestVaultIdentitySaveUpdatesSelectorAndPreservesSection(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	info := storage.VaultInfo{ID: "personal", Name: "Personal", Appearance: storage.DefaultVaultAppearance()}
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) { return []storage.VaultInfo{info}, nil }
	store.OnUpdateVaultIdentity = func(got *model.Vault, name string, appearance storage.VaultAppearance) error {
		require.Same(t, vault, got)
		vault.Metadata.VaultName = name
		info.Name, info.Appearance = name, appearance
		return nil
	}
	a := &app{win: window, main: container.NewVScroll(nil), storage: store, vault: vault, vaultID: "personal", vaultSection: 1, unlockedVault: map[string]*model.Vault{"personal": vault}, filter: make(map[string]*paw.VaultFilterOptions)}
	a.showVaultSettingsView()
	settings := window.Content()
	test.Tap(findVaultSettingsRow(t, settings))
	editor := window.Content()
	findVaultIdentityName(t, editor).SetText("Work")
	test.Tap(findVaultIdentitySave(t, editor))
	require.Same(t, a.main, window.Content())
	require.Equal(t, "Work", info.Name)
	require.Equal(t, "Work", vault.Metadata.VaultName)
	var selected int = -1
	updatedHeader := false
	for _, object := range test.LaidOutObjects(a.main) {
		if navigation, ok := object.(*Navigation); ok {
			selected = navigation.Selected
		}
		if label, ok := object.(*widget.Label); ok && label.Text == "Work" {
			updatedHeader = true
		}
	}
	require.Equal(t, 1, selected)
	require.True(t, updatedHeader)
}

func TestVaultSettingsCannotSwitchToAnotherVault(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	current, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	a := &app{win: window, vault: current, vaultID: "personal", settingsVaultID: "personal", unlockedVault: map[string]*model.Vault{"personal": current}}
	a.setVaultViewByInfo(storage.VaultInfo{ID: "work", Name: "Work"}, nil)
	require.Same(t, current, a.vault)
	require.Equal(t, "personal", a.vaultID)
}

func TestVaultIdentityEditorSavesSelectedAppearanceAndRejectsBlankName(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			var savedName string
			var savedAppearance storage.VaultAppearance
			view := NewVaultIdentityEditor("Personal", storage.DefaultVaultAppearance(), func(name string, appearance storage.VaultAppearance) error {
				savedName, savedAppearance = name, appearance
				return nil
			}, func() {})
			window := test.NewTempWindow(t, view)
			window.Resize(fyne.NewSize(320, 540))
			name := findVaultIdentityName(t, view)
			save := findVaultIdentitySave(t, view)
			name.SetText("  ")
			require.True(t, save.Disabled())
			name.SetText("Work")
			require.False(t, save.Disabled())

			preview := findVaultIdentityPreview(t, view)
			test.Tap(preview)
			appearanceView := window.Content()
			for _, object := range test.LaidOutObjects(appearanceView) {
				button, ok := object.(*widget.Button)
				if ok && button.Icon != nil && strings.Contains(button.Icon.Name(), "briefcase-business") {
					test.Tap(button)
					break
				}
			}
			for _, object := range test.LaidOutObjects(appearanceView) {
				circle, ok := object.(*canvas.Circle)
				if ok && circle.FillColor == selectedVaultColor("green") {
					position := app.Driver().AbsolutePositionForObject(circle).Add(fyne.NewPos(circle.Size().Width/2, circle.Size().Height/2))
					test.TapCanvas(window.Canvas(), position)
					break
				}
			}
			for _, object := range test.LaidOutObjects(appearanceView) {
				if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
					test.Tap(button)
					break
				}
			}
			identityView := window.Content()
			require.Equal(t, "Work", findVaultIdentityName(t, identityView).Text)
			test.Tap(findVaultIdentitySave(t, identityView))
			require.Equal(t, "Work", savedName)
			require.Equal(t, storage.VaultAppearance{Icon: "briefcase-business", Color: "green"}, savedAppearance)
		})
	}
}

func findVaultSettingsRow(t *testing.T, view fyne.CanvasObject) *widget.Button {
	t.Helper()
	return findNavigationRowButton(t, view, "Personal")
}

func findNavigationRowButton(t *testing.T, view fyne.CanvasObject, title string) *widget.Button {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if row, ok := object.(*fyne.Container); ok && len(row.Objects) == 2 {
			if button, ok := row.Objects[0].(*widget.Button); ok {
				for _, child := range test.LaidOutObjects(row.Objects[1]) {
					if label, ok := child.(*widget.Label); ok && label.Text == title {
						return button
					}
				}
			}
		}
	}
	t.Fatal("vault settings identity row not found")
	return nil
}

func findVaultIdentityName(t *testing.T, view fyne.CanvasObject) *widget.Entry {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok {
			return entry
		}
	}
	t.Fatal("vault name field not found")
	return nil
}

func findVaultIdentitySave(t *testing.T, view fyne.CanvasObject) *widget.Button {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Save" {
			return button
		}
	}
	t.Fatal("Save button not found")
	return nil
}

func findVaultIdentityPreview(t *testing.T, view fyne.CanvasObject) *widget.Button {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "" && button.Icon == nil && button.Size().Width >= 80 {
			return button
		}
	}
	t.Fatal("vault appearance preview not found")
	return nil
}
