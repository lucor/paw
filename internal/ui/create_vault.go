// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// NewCreateVault displays the vault name and master password form.
func NewCreateVault(back func(), create func(name, password string, appearance model.VaultAppearance) error) fyne.CanvasObject {
	appearance := model.DefaultVaultAppearance()
	name := widget.NewEntry()
	name.SetPlaceHolder("e.g. Personal")
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New("vault name is required")
		}
		return nil
	}
	nameHint := newText("Identify this vault on your devices.", captionStyle)
	nameInput := container.NewVBox(name, nameHint)
	passwordField := NewPasswordField()
	password := passwordField.Entry
	confirmValidator := func(value string) error {
		if value == "" {
			return errors.New("password confirmation is required")
		}
		if value != password.Text {
			return errors.New("passwords do not match")
		}
		return nil
	}
	confirm := widget.NewPasswordEntry()
	confirm.Validator = confirmValidator
	message := newErrorText("")
	message.Hide()
	showError := func(text string) {
		message.SetText(text)
		message.Show()
	}

	form := widget.NewForm(
		widget.NewFormItem("Vault name", nameInput),
		widget.NewFormItem("Master password", passwordField),
		widget.NewFormItem("Confirm password", confirm),
	)
	form.Orientation = widget.Vertical
	createButton := widget.NewButton("Create vault", func() {
		if err := name.Validate(); err != nil || form.Validate() != nil || strings.TrimSpace(name.Text) == "" || password.Text == "" || confirm.Text != password.Text {
			return
		}
		message.Hide()
		if err := create(name.Text, password.Text, appearance); err != nil {
			showError(err.Error())
		}
	})
	createButton.Importance = widget.HighImportance
	createButton.Disable()
	updateSubmit := func() {
		if strings.TrimSpace(name.Text) == "" || password.Text == "" || confirm.Text != password.Text {
			createButton.Disable()
			return
		}
		createButton.Enable()
	}
	name.OnChanged = func(string) { updateSubmit() }
	passwordWasEmpty := true
	passwordField.OnChanged = func(value string) {
		empty := value == ""
		if empty != passwordWasEmpty {
			form.Refresh()
			passwordWasEmpty = empty
		}
		if confirm.Text != "" {
			confirm.Validate()
		}
		updateSubmit()
	}
	confirm.OnChanged = func(string) { updateSubmit() }

	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	heading := newText("Create new vault", titleStyle)
	header := newPageHeader(backButton, heading, nil)
	footer := container.NewVBox(message, container.NewStack(createButton))

	previewButton := widget.NewButton("", nil)
	previewButton.Importance = widget.LowImportance
	preview := func() fyne.CanvasObject {
		return newEditableVaultPreview(appearance, previewButton)
	}
	previewObject := preview()
	fields := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)),
		container.New(layout.NewCustomPaddedLayout(24, 24, 0, 0), container.NewCenter(previewObject)),
		form,
	)
	formPage := newSecondaryPage(header, footer, container.NewVScroll(fields))
	screen := container.NewStack(formPage)
	previewButton.OnTapped = func() {
		appearancePage := newVaultAppearanceView(name.Text, appearance, func(updated model.VaultAppearance) {
			appearance = updated
			previewObject = preview()
			fields.Objects[0] = container.New(layout.NewCustomPaddedLayout(24, 24, 0, 0), container.NewCenter(previewObject))
			fields.Refresh()
			updateSubmit()
		}, func() {
			screen.Objects = []fyne.CanvasObject{formPage}
			screen.Refresh()
		})
		screen.Objects = []fyne.CanvasObject{appearancePage}
		screen.Refresh()
	}
	return screen
}
