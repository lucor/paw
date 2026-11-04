// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// NewAddVault shows the available ways to add a vault.
func NewAddVault(back, create, restore func()) fyne.CanvasObject {
	var backButton fyne.CanvasObject
	if back != nil {
		button := widget.NewButtonWithIcon("", icons.Back, back)
		button.Importance = widget.LowImportance
		backButton = button
	}
	header := newPageHeader(backButton, newText("Add vault", titleStyle), nil)
	createRow := newNavigationRowWithSubtitle("Create new vault", "Start with an empty encrypted vault", navigationRowIcon(icons.PlusCircle), create)
	restoreRow := newNavigationRowWithSubtitle("Restore from backup", "Create a vault from a backup file", navigationRowIcon(icons.ArchiveRestore), restore)
	choices := container.New(layout.NewVBoxLayout(), createRow, widget.NewSeparator(), restoreRow)
	return newSecondaryPage(header, nil, container.NewVScroll(choices))
}

// ShowAddVault opens the choice page and keeps Back navigation within the flow.
func ShowAddVault(window fyne.Window, back func(), create func(string, string, model.VaultAppearance) error, restore func()) {
	showChoices := func() { ShowAddVault(window, back, create, restore) }
	window.SetContent(NewAddVault(back,
		func() { window.SetContent(NewCreateVault(showChoices, create)) },
		restore,
	))
}
