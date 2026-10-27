// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// NewUnlockVault lays out the controls owned by the vault unlock flow.
func NewUnlockVault(name string, password *widget.Entry, unlock *widget.Button, back, addVault func()) fyne.CanvasObject {
	heading := newText("Unlock "+name, headingStyle)
	heading.Alignment = fyne.TextAlignCenter
	var navigation fyne.CanvasObject
	if back != nil {
		button := widget.NewButtonWithIcon("", icons.Back, back)
		button.Importance = widget.LowImportance
		navigation = button
	}
	more := newVaultOptions(addVault)
	header := container.NewBorder(nil, nil, navigation, more)
	backdrop := theme.NewColoredResource(fyne.NewStaticResource("vault-backdrop.svg", []byte(
		`<svg xmlns="http://www.w3.org/2000/svg" width="96" height="96" viewBox="0 0 96 96"><circle cx="48" cy="48" r="48" fill="#000000"/></svg>`)), theme.ColorNameFocus)
	badge := container.NewStack(
		container.NewGridWrap(fyne.NewSquareSize(96), widget.NewIcon(backdrop)),
		container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(32), widget.NewIcon(icons.Lock))),
	)
	fields := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(24, 24, 0, 0), container.NewCenter(badge)),
		heading,
		password,
		container.NewStack(unlock),
	)
	content := container.NewVBox(layout.NewSpacer(), fields, layout.NewSpacer())
	page := container.NewBorder(header, nil, nil, nil, container.NewVScroll(content))
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), page)
}

// newVaultOptions provides the shared menu for opening the Add vault page.
func newVaultOptions(addVault func()) *widget.Button {
	more := widget.NewButtonWithIcon("", icons.More, nil)
	more.Importance = widget.LowImportance
	menu := fyne.NewMenu("", fyne.NewMenuItem("Add vault", addVault))
	more.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(menu, fyne.CurrentApp().Driver().CanvasForObject(more),
			fyne.NewPos(0, more.Size().Height), more)
	}
	return more
}
