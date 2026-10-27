// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package v2 contains Paw's redesigned Fyne interface.
package v2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// WelcomeActions connects the welcome screen to the available flows.
type WelcomeActions struct {
	CreateVault func()
	Restore     func()
	Sync        func()
}

// NewWelcome creates the first-run screen using native Fyne widgets.
func NewWelcome(logo fyne.CanvasObject, actions WelcomeActions) fyne.CanvasObject {
	heading := newText("Welcome to Paw", headingStyle)
	heading.Alignment = fyne.TextAlignCenter
	subtitle := newText("Your passwords, stored securely\non your device.", subtitleStyle)
	subtitle.Alignment = fyne.TextAlignCenter
	footnote := newText("Your vault stays under your control.", helpStyle)
	footnote.Alignment = fyne.TextAlignCenter
	rows := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)),
		welcomeAction("Create new vault", "Start with an empty encrypted vault", icons.CreateVault, actions.CreateVault),
		welcomeAction("Restore from backup", "Open an existing vault file", icons.RestoreVault, actions.Restore),
		welcomeAction("Connect via Sync", "Pair with another device to receive a vault", icons.Sync, actions.Sync),
	)

	content := container.NewVBox(
		container.NewCenter(logo),
		heading,
		subtitle,
		rows,
		widget.NewSeparator(),
		footnote,
	)
	return container.NewStack(container.NewPadded(container.NewVBox(
		layout.NewSpacer(),
		content,
		layout.NewSpacer(),
	)))
}

// NewPlaceholder shows a temporary section with a way back to the welcome screen.
func NewPlaceholder(title string, resource fyne.Resource, back func()) fyne.CanvasObject {
	button := widget.NewButtonWithIcon("Back", icons.Back, back)
	header := container.NewBorder(nil, nil, button, nil,
		newText(title, viewTitleStyle))
	heading := newText(title, headingStyle)
	heading.Alignment = fyne.TextAlignCenter
	content := container.NewCenter(container.NewVBox(
		container.NewCenter(widget.NewIcon(resource)),
		heading,
		newText("This section is not available yet.", bodyStyle),
	))
	return container.NewBorder(header, nil, nil, nil, content)
}

// welcomeAction layers the row content over one native button.
func welcomeAction(title, description string, resource fyne.Resource, tapped func()) fyne.CanvasObject {
	return NewActionRow(title, description, resource, tapped)
}
