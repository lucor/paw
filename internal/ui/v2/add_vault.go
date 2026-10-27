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

// NewAddVault shows the available ways to add a vault.
func NewAddVault(back, create, restore, sync func()) fyne.CanvasObject {
	button := widget.NewButtonWithIcon("", icons.Back, back)
	button.Importance = widget.LowImportance
	header := container.NewBorder(nil, nil, button, nil, newText("Add vault", viewTitleStyle))
	choices := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)),
		welcomeAction("Create new vault", "Start with an empty encrypted vault", icons.CreateVault, create),
		welcomeAction("Restore from backup", "Open an existing vault file", icons.RestoreVault, restore),
		welcomeAction("Connect via Sync", "Pair with another device to receive a vault", icons.Sync, sync),
	)
	page := container.NewBorder(header, nil, nil, nil, container.NewVScroll(choices))
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), page)
}

// ShowAddVault opens the choice page and keeps Back navigation within the flow.
func ShowAddVault(window fyne.Window, back func(), create func(string, string) error) {
	showChoices := func() { ShowAddVault(window, back, create) }
	placeholder := func(title string, resource fyne.Resource) {
		window.SetContent(NewPlaceholder(title, resource, showChoices))
	}
	window.SetContent(NewAddVault(back,
		func() { window.SetContent(NewCreateVault(showChoices, create)) },
		func() { placeholder("Restore from backup", icons.RestoreVault) },
		func() { placeholder("Connect via Sync", icons.Sync) },
	))
}
