// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// NewVaultSettingsView displays settings for the currently selected vault.
func NewVaultSettingsView(name string, appearance storage.VaultAppearance, edit, changePassword, deleteVault, back func()) fyne.CanvasObject {
	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	header := newPageHeader(backButton, newText("Vault settings", titleStyle), nil)

	row := newNavigationRowWithSubtitle(name, "Change name, icon and color", newVaultIcon(appearance, vaultIconMedium), edit)
	securityIcon := container.NewGridWrap(fyne.NewSquareSize(40), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(20), widget.NewIcon(icons.Password))))
	dangerIcon := container.NewGridWrap(fyne.NewSquareSize(40), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(20), widget.NewIcon(icons.Delete))))
	security := newNavigationRow("Change master password", securityIcon, changePassword)
	danger := newNavigationRow("Delete vault", dangerIcon, deleteVault)
	securityHeading := newText("Security", captionStyle)
	securityHeading.TextStyle = fyne.TextStyle{Bold: true}
	dangerHeading := newText("Danger zone", captionStyle)
	dangerHeading.TextStyle = fyne.TextStyle{Bold: true}
	content := container.NewVBox(row, widget.NewSeparator(),
		container.New(layout.NewCustomPaddedVBoxLayout(0), securityHeading, security),
		widget.NewSeparator(),
		container.New(layout.NewCustomPaddedVBoxLayout(0), dangerHeading, danger))
	return newSecondaryPage(header, nil, container.NewVScroll(content))
}

// NewVaultIdentityEditor keeps name and appearance changes temporary until Save.
func NewVaultIdentityEditor(name string, appearance storage.VaultAppearance, save func(string, storage.VaultAppearance) error, back func()) fyne.CanvasObject {
	appearance = storage.NormalizeVaultAppearance(appearance)

	nameField := widget.NewEntry()
	nameField.SetText(name)
	nameField.SetPlaceHolder("e.g. Personal")

	saveButton := widget.NewButton("Save", nil)
	saveButton.Importance = widget.HighImportance
	saveButton.Disable()
	errorText := newErrorText("")
	errorText.Hide()
	updateSave := func() {
		if strings.TrimSpace(nameField.Text) == "" {
			saveButton.Disable()
			return
		}
		saveButton.Enable()
	}
	nameField.OnChanged = func(string) {
		errorText.Hide()
		updateSave()
	}
	saveButton.OnTapped = func() {
		newName := strings.TrimSpace(nameField.Text)
		if newName == "" {
			updateSave()
			return
		}
		if err := save(newName, appearance); err != nil {
			errorText.SetText(err.Error())
			errorText.Show()
		}
	}
	updateSave()

	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	header := newPageHeader(backButton, newText("Edit vault", titleStyle), nil)

	previewButton := widget.NewButton("", nil)
	previewButton.Importance = widget.LowImportance
	preview := container.NewCenter(newEditableVaultPreview(appearance, previewButton))
	form := widget.NewForm(widget.NewFormItem("Vault name", nameField))
	form.Orientation = widget.Vertical
	fields := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)),
		preview,
		form,
	)
	footer := container.NewVBox(errorText, container.NewStack(saveButton))
	editor := newSecondaryPage(header, footer, container.NewVScroll(fields))
	screen := container.NewStack(editor)
	previewButton.OnTapped = func() {
		errorText.Hide()
		appearancePage := newVaultAppearanceView(nameField.Text, appearance, func(updated storage.VaultAppearance) {
			appearance = updated
			preview.Objects[0] = newEditableVaultPreview(appearance, previewButton)
			preview.Refresh()
		}, func() {
			screen.Objects = []fyne.CanvasObject{editor}
			screen.Refresh()
		})
		screen.Objects = []fyne.CanvasObject{appearancePage}
		screen.Refresh()
	}
	return screen
}

func (a *app) showVaultSettingsView() {
	if a.vault == nil || a.vaultID == "" {
		return
	}
	a.closeItemDetail()
	a.settingsVaultID = a.vaultID
	a.showVaultSettings()
}

func (a *app) showVaultSettings() {
	if a.vault == nil || a.vaultID == "" || a.settingsVaultID != a.vaultID {
		a.settingsVaultID = ""
		a.showCurrentVaultView()
		return
	}
	name, err := a.vault.Name()
	if err != nil {
		a.settingsVaultID = ""
		dialog.ShowError(err, a.win)
		return
	}
	vaultID := a.vaultID
	infos, err := a.storage.Vaults()
	if err != nil {
		a.settingsVaultID = ""
		dialog.ShowError(err, a.win)
		return
	}
	appearance := storage.DefaultVaultAppearance()
	for _, info := range infos {
		if info.ID == vaultID {
			appearance = info.Appearance
			break
		}
	}
	view := NewVaultSettingsView(name, appearance, func() { a.showVaultIdentityEditor(vaultID, name, appearance) }, a.showChangeMasterPassword, func() {
		a.confirmDeleteVault(vaultID, name)
	}, func() {
		a.settingsVaultID = ""
		a.showCurrentVaultView()
	})
	a.win.SetContent(view)
}

func (a *app) confirmDeleteVault(vaultID, name string) {
	if a.vault == nil || a.vaultID != vaultID || a.settingsVaultID != vaultID || a.deletingVault {
		return
	}
	showDeleteConfirmation("Delete vault?", "Delete vault", fmt.Sprintf("Permanently delete %q and its locally stored data from this device?\nThis action cannot be undone.", name), func() {
		if a.vault == nil || a.vaultID != vaultID || a.settingsVaultID != vaultID || a.deletingVault {
			return
		}
		a.deletingVault = true
		defer func() { a.deletingVault = false }()
		if err := a.storage.DeleteVault(vaultID); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		delete(a.unlockedVault, vaultID)
		vaults, err := a.storage.Vaults()
		if err != nil {
			a.main.Content = NewAddVault(nil, a.showCreateVaultView, a.showRestoreVaultView)
			a.lockVault()
			dialog.ShowError(err, a.win)
			return
		}
		if len(vaults) == 0 {
			a.main.Content = NewAddVault(nil, a.showCreateVaultView, a.showRestoreVaultView)
		} else {
			a.main.Content = a.makeSelectVaultView(vaults)
		}
		a.lockVault()
		a.main.Refresh()
	}, a.win)
}

func (a *app) showVaultIdentityEditor(vaultID, name string, appearance storage.VaultAppearance) {
	if a.vault == nil || a.vaultID != vaultID || a.settingsVaultID != vaultID {
		a.settingsVaultID = ""
		a.showCurrentVaultView()
		return
	}
	view := NewVaultIdentityEditor(name, appearance, func(updatedName string, updatedAppearance storage.VaultAppearance) error {
		if a.vault == nil || a.vaultID != vaultID || a.settingsVaultID != vaultID {
			return errors.New("the selected vault is no longer available")
		}
		if err := a.storage.UpdateVaultIdentity(a.vault, updatedName, updatedAppearance); err != nil {
			return err
		}
		a.settingsVaultID = ""
		a.main.Content = a.makeCurrentVaultView()
		a.main.Refresh()
		a.showCurrentVaultView()
		return nil
	}, func() {
		a.showVaultSettings()
	})
	a.win.SetContent(view)
}
