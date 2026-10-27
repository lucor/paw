// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// NewCreateVault displays the vault name and master password form.
func NewCreateVault(back func(), create func(name, password string) error) fyne.CanvasObject {
	name := widget.NewEntry()
	name.SetPlaceHolder("e.g. Personal")
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" {
			return errors.New("vault name is required")
		}
		return nil
	}
	password := widget.NewPasswordEntry()
	password.Validator = func(value string) error {
		if value == "" {
			return errors.New("master password is required")
		}
		return nil
	}
	confirm := widget.NewPasswordEntry()
	confirm.Validator = func(value string) error {
		if value == "" {
			return errors.New("password confirmation is required")
		}
		if value != password.Text {
			return errors.New("passwords do not match")
		}
		return nil
	}
	message := newText("", errorStyle)
	message.Hide()
	showError := func(text string) {
		message.SetText(text)
		message.Show()
	}

	form := widget.NewForm(
		widget.NewFormItem("Vault name", name),
		widget.NewFormItem("Master password", password),
		widget.NewFormItem("Confirm password", confirm),
	)
	form.Orientation = widget.Vertical
	form.Items[0].HintText = "Choose a name to easily identify this vault on your devices."
	createButton := widget.NewButton("Create vault", func() {
		if err := form.Validate(); err != nil {
			return
		}
		message.Hide()
		if err := create(name.Text, password.Text); err != nil {
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
	password.OnChanged = func(string) { updateSubmit() }
	confirm.OnChanged = func(string) { updateSubmit() }

	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	heading := newText("Create new vault", viewTitleStyle)
	backdrop := theme.NewColoredResource(fyne.NewStaticResource("vault-backdrop.svg", []byte(
		`<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" viewBox="0 0 96 96"><circle cx="48" cy="48" r="48" fill="#000000"/></svg>`)), theme.ColorNameFocus)
	badge := container.NewStack(
		container.NewGridWrap(fyne.NewSquareSize(96), widget.NewIcon(backdrop)),
		container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(32), widget.NewIcon(icons.Vault))),
	)
	fields := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)),
		container.New(layout.NewCustomPaddedLayout(24, 24, 0, 0), container.NewCenter(badge)),
		form,
	)
	header := container.NewBorder(nil, nil, backButton, nil, heading)
	footer := container.NewVBox(message, container.NewStack(createButton))
	page := container.NewBorder(header, footer, nil, nil, container.NewVScroll(fields))
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), page)
}
