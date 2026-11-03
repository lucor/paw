// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
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
	view := NewVaultSettingsView("Personal", model.DefaultVaultAppearance(), func() { editCalled = true }, func() {}, func() {}, func() { backCalled = true })
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
	view := NewVaultSettingsView("Personal", model.DefaultVaultAppearance(), func() {}, func() {}, func() { deleteRequested = true }, func() {})
	test.Tap(findNavigationRowButton(t, view, "Delete vault"))
	require.True(t, deleteRequested)
}

func TestDeleteVaultConfirmationUsesNamedWarningAndTextOnlyActions(t *testing.T) {
	testApp := test.NewTempApp(t)
	window := test.NewTempWindow(t, container.NewVBox())
	confirmed := false
	showDeleteConfirmation("Delete vault?", "Delete vault", "Permanently delete \"Personal\" and its locally stored data from this device?\nThis action cannot be undone.", func() { confirmed = true }, window)

	objects := test.LaidOutObjects(window.Canvas().Overlays().Top())
	labels := make([]string, 0)
	buttons := make(map[string]*widget.Button)
	trashIcon := false
	var icon *widget.Icon
	var title *widget.Label
	for _, object := range objects {
		if label, ok := object.(*widget.Label); ok {
			labels = append(labels, label.Text)
			if label.Text == "Delete vault?" {
				title = label
			}
		}
		if button, ok := object.(*widget.Button); ok {
			buttons[button.Text] = button
		}
		if iconObject, ok := object.(*widget.Icon); ok && strings.Contains(iconObject.Resource.Name(), "trash.png") {
			trashIcon = true
			icon = iconObject
		}
	}
	require.Contains(t, labels, "Delete vault?")
	require.Contains(t, labels, "Permanently delete \"Personal\" and its locally stored data from this device?\nThis action cannot be undone.")
	require.True(t, trashIcon, "the title includes Paw's trash icon")
	require.NotNil(t, icon)
	require.NotNil(t, title)
	iconCenter := testApp.Driver().AbsolutePositionForObject(icon).Add(fyne.NewPos(icon.Size().Width/2, icon.Size().Height/2)).Y
	titleCenter := testApp.Driver().AbsolutePositionForObject(title).Add(fyne.NewPos(0, title.Size().Height/2)).Y
	require.InDelta(t, titleCenter, iconCenter, 1, "the icon and title should be vertically centered together")
	require.Contains(t, buttons, "Cancel")
	require.Contains(t, buttons, "Delete vault")
	require.Nil(t, buttons["Cancel"].Icon)
	require.Equal(t, widget.LowImportance, buttons["Cancel"].Importance)
	require.Nil(t, buttons["Delete vault"].Icon)
	require.Equal(t, widget.DangerImportance, buttons["Delete vault"].Importance)
	require.False(t, buttons["Delete vault"] == window.Canvas().Focused(), "the destructive action must not receive focus automatically")
	require.False(t, confirmed, "opening the dialog must not trigger deletion")

	test.Tap(buttons["Cancel"])
	require.False(t, confirmed)
	require.Nil(t, window.Canvas().Overlays().Top())

	showDeleteConfirmation("Delete vault?", "Delete vault", "Permanently delete \"Personal\" and its locally stored data from this device?\nThis action cannot be undone.", func() { confirmed = true }, window)
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Delete vault" {
			test.Tap(button)
			break
		}
	}
	require.True(t, confirmed)
	require.Nil(t, window.Canvas().Overlays().Top())
}

func TestDeleteVaultConfirmationDismissalDoesNotDelete(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, container.NewVBox())
	deleted := false
	confirmation := showDeleteConfirmation("Delete vault?", "Delete vault", "Permanently delete \"Personal\" and its locally stored data from this device?\nThis action cannot be undone.", func() { deleted = true }, window)
	confirmation.Hide()
	require.False(t, deleted, "closing the dialog must behave like Cancel")
	require.Nil(t, window.Canvas().Overlays().Top())
}

func TestDeleteVaultSuccessNavigatesToRemainingVaultSelector(t *testing.T) {
	testDeleteVaultNavigation(t, []model.VaultInfo{{ID: "work", Name: "Work"}}, nil, "Your vaults")
}

func TestDeleteLastVaultSuccessNavigatesToAddVault(t *testing.T) {
	testDeleteVaultNavigation(t, nil, nil, "Create new vault")
}

func TestDeleteVaultFailureKeepsVaultAndShowsError(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	failure := errors.New("storage unavailable")
	store := &storage.StorageMock{
		VaultStorageMock: storage.VaultStorageMock{
			OnVaults: func() ([]model.VaultInfo, error) {
				return []model.VaultInfo{{ID: "personal", Name: "Personal"}}, nil
			},
			OnDeleteVault: func(string) error { return failure },
		},
	}
	a := &app{win: window, main: container.NewVScroll(nil), storage: store, vault: vault, vaultID: "personal", settingsVaultID: "personal", unlockedVault: map[string]*model.Vault{"personal": vault}}
	a.confirmDeleteVault("personal", "Personal")
	deleteVaultConfirmationButton(t, window, "Delete vault")
	require.Same(t, vault, a.vault)
	require.Equal(t, "personal", a.vaultID)
	require.Equal(t, "personal", a.settingsVaultID)
	require.False(t, a.deletingVault)
	require.NotNil(t, window.Canvas().Overlays().Top(), "the existing error dialog should be shown")
}

func testDeleteVaultNavigation(t *testing.T, remaining []model.VaultInfo, deletionErr error, expectedLabel string) {
	t.Helper()
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	infos := []model.VaultInfo{{ID: "personal", Name: "Personal"}}
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]model.VaultInfo, error) { return infos, nil }
	a := &app{win: window, main: container.NewVScroll(nil), storage: store, vault: vault, vaultID: "personal", settingsVaultID: "personal", unlockedVault: map[string]*model.Vault{"personal": vault}}
	deleteCalls := 0
	store.OnDeleteVault = func(string) error {
		deleteCalls++
		a.confirmDeleteVault("personal", "Personal")
		infos = remaining
		return deletionErr
	}
	a.confirmDeleteVault("personal", "Personal")
	deleteVaultConfirmationButton(t, window, "Delete vault")
	require.Equal(t, 1, deleteCalls)
	require.Nil(t, a.vault)
	require.Empty(t, a.vaultID)
	require.NotContains(t, a.unlockedVault, "personal")
	require.False(t, a.deletingVault)
	var found bool
	for _, object := range test.LaidOutObjects(a.main) {
		if label, ok := object.(*widget.Label); ok && label.Text == expectedLabel {
			found = true
			break
		}
	}
	require.True(t, found, "expected navigation content %q", expectedLabel)
}

func deleteVaultConfirmationButton(t *testing.T, window fyne.Window, text string) {
	t.Helper()
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if button, ok := object.(*widget.Button); ok && button.Text == text {
			test.Tap(button)
			return
		}
	}
	t.Fatalf("confirmation action %q not found", text)
}

func TestVaultIdentitySaveUpdatesSelectorAndPreservesSection(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	info := model.VaultInfo{ID: "personal", Name: "Personal", Appearance: model.DefaultVaultAppearance()}
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]model.VaultInfo, error) { return []model.VaultInfo{info}, nil }
	store.OnUpdateVaultIdentity = func(got *model.Vault, name string, appearance model.VaultAppearance) error {
		require.Same(t, vault, got)
		vault.Metadata.VaultName = name
		info.Name, info.Appearance = name, appearance
		return nil
	}
	a := &app{win: window, main: container.NewVScroll(nil), storage: store, vault: vault, vaultID: "personal", vaultSection: 1, unlockedVault: map[string]*model.Vault{"personal": vault}, filter: make(map[string]*vaultFilterOptions)}
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
	a.setVaultViewByInfo(model.VaultInfo{ID: "work", Name: "Work"}, nil, nil)
	require.Same(t, current, a.vault)
	require.Equal(t, "personal", a.vaultID)
}

func TestVaultIdentityEditorSavesSelectedAppearanceAndRejectsBlankName(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			var savedName string
			var savedAppearance model.VaultAppearance
			view := NewVaultIdentityEditor("Personal", model.DefaultVaultAppearance(), func(name string, appearance model.VaultAppearance) error {
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
			require.Equal(t, model.VaultAppearance{Icon: "briefcase-business", Color: "green"}, savedAppearance)
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
