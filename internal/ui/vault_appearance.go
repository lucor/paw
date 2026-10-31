// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type vaultColorOption struct {
	id string
}

var vaultColors = []vaultColorOption{
	{id: "none"},
	{id: "gray"},
	{id: "blue"},
	{id: "green"},
	{id: "purple"},
	{id: "orange"},
	{id: "red"},
}

var vaultIcons = []struct {
	id       string
	resource fyne.Resource
}{
	{id: "paw", resource: icons.Paw},
	{id: "briefcase-business", resource: icons.BriefcaseBusiness},
	{id: "house", resource: icons.House},
	{id: "archive", resource: icons.Archive},
	{id: "shield", resource: icons.Shield},
	{id: "folder", resource: icons.Folder},
	{id: "key-round", resource: icons.Password},
	{id: "laptop", resource: icons.Laptop},
}

func newVaultAppearanceView(name string, appearance storage.VaultAppearance, changed func(storage.VaultAppearance), back func()) fyne.CanvasObject {
	appearance = storage.NormalizeVaultAppearance(appearance)
	previewIcon := newVaultIcon(appearance, vaultIconMedium)
	previewName := newText(name, titleStyle)
	previewCaption := newText("Vault preview", captionStyle)
	if previewName.Text == "" {
		previewName.SetText("Vault preview")
		previewCaption.Hide()
	}
	preview := func() *fyne.Container {
		return container.New(layout.NewCustomPaddedHBoxLayout(0), previewIcon, container.NewVBox(previewName, previewCaption))
	}
	previewRow := preview()

	iconButtons := make([]fyne.CanvasObject, 0, len(vaultIcons))
	var refreshPreview func()
	refreshIcons := func() {
		iconButtons = iconButtons[:0]
		for _, option := range vaultIcons {
			choice := option
			button := widget.NewButtonWithIcon("", choice.resource, func() {
				appearance.Icon = choice.id
				refreshPreview()
			})
			button.Importance = widget.LowImportance
			if choice.id == appearance.Icon {
				button.Importance = widget.MediumImportance
			}
			iconButtons = append(iconButtons, container.NewGridWrap(fyne.NewSize(64, 44), button))
		}
	}
	colorButtons := make([]fyne.CanvasObject, 0, len(vaultColors))
	refreshColors := func() {
		colorButtons = colorButtons[:0]
		for _, option := range vaultColors {
			choice := option
			button := widget.NewButton("", func() {
				appearance.Color = choice.id
				refreshPreview()
			})
			button.Importance = widget.LowImportance
			var swatch fyne.CanvasObject
			fill := selectedVaultColor(choice.id)
			if choice.id == "none" {
				circle := canvas.NewCircle(fill)
				circle.StrokeColor = theme.Color(theme.ColorNameForeground)
				circle.StrokeWidth = 1
				swatch = circle
			} else {
				swatch = canvas.NewCircle(fill)
			}
			if choice.id == appearance.Color {
				outer := canvas.NewCircle(theme.Color(theme.ColorNameBackground))
				outer.StrokeColor = theme.Color(theme.ColorNameForeground)
				outer.StrokeWidth = 1
				if choice.id == "none" {
					swatch = container.NewStack(outer, container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(16), widget.NewIcon(icons.Check))))
				} else {
					disk := canvas.NewCircle(fill)
					swatch = container.NewStack(outer, container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(24), disk)), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(16), widget.NewIcon(icons.Check))))
				}
			} else {
				swatch = container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(28), swatch))
			}
			colorButtons = append(colorButtons, container.NewGridWrap(fyne.NewSquareSize(32), container.NewStack(button, swatch)))
		}
	}

	var iconGrid, colorRow *fyne.Container
	refreshPreview = func() {
		appearance = storage.NormalizeVaultAppearance(appearance)
		previewIcon = newVaultIcon(appearance, vaultIconMedium)
		previewRow.Objects[0] = previewIcon
		previewName.SetText(name)
		if name == "" {
			previewName.SetText("Vault preview")
			previewCaption.Hide()
		} else {
			previewCaption.Show()
		}
		refreshIcons()
		refreshColors()
		iconGrid.Objects = iconButtons
		colorRow.Objects = colorButtons
		previewRow.Refresh()
		iconGrid.Refresh()
		colorRow.Refresh()
		if changed != nil {
			changed(appearance)
		}
	}
	refreshIcons()
	refreshColors()
	iconGrid = container.NewGridWithColumns(4, iconButtons...)
	colorRow = container.NewHBox(colorButtons...)

	headerBack := widget.NewButtonWithIcon("", icons.Back, back)
	headerBack.Importance = widget.LowImportance
	header := newPageHeader(headerBack, newText("Vault appearance", titleStyle), nil)
	content := container.NewVBox(
		container.NewPadded(previewRow),
		widget.NewSeparator(),
		newText("Icon", captionStyle),
		iconGrid,
		newText("Color", captionStyle),
		colorRow,
	)
	return newSecondaryPage(header, nil, container.NewVScroll(content))
}
