// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const (
	navigationRowIconAreaSize     = 40
	navigationRowIconSize         = 20
	navigationRowTrailingIconSize = 16
)

func navigationRowIcon(resource fyne.Resource) fyne.CanvasObject {
	if favicon, ok := resource.(*model.Favicon); ok && len(favicon.Data) > 0 {
		preview := container.NewGridWrap(fyne.NewSquareSize(navigationRowIconSize), newIconPreviewWithIconSize(favicon, smallFaviconIconSize))
		return container.NewGridWrap(fyne.NewSquareSize(navigationRowIconAreaSize), container.NewCenter(preview))
	}
	return container.NewGridWrap(fyne.NewSquareSize(navigationRowIconAreaSize), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(navigationRowIconSize), widget.NewIcon(resource))))
}

// newNavigationRow creates a clickable row with one text line.
func newNavigationRow(title string, leading fyne.CanvasObject, tapped func()) fyne.CanvasObject {
	return newNavigationRowWithIcon(title, "", leading, icons.Next, tapped)
}

// newNavigationRowWithSubtitle adds an optional secondary line to the same layout.
func newNavigationRowWithSubtitle(title, subtitle string, leading fyne.CanvasObject, tapped func()) fyne.CanvasObject {
	return newNavigationRowWithIcon(title, subtitle, leading, icons.Next, tapped)
}

func newNavigationRowWithTrailingIcon(title string, leading fyne.CanvasObject, trailing fyne.Resource, tapped func()) fyne.CanvasObject {
	return newNavigationRowWithIcon(title, "", leading, trailing, tapped)
}

func newNavigationRowWithIcon(title, subtitle string, leading fyne.CanvasObject, trailing fyne.Resource, tapped func()) fyne.CanvasObject {
	label := newText(title, bodyStyle)
	label.TextStyle.Bold = true
	text := container.NewVBox(label)
	if subtitle != "" {
		text.Add(newText(subtitle, captionStyle))
	}
	compact := container.NewThemeOverride(container.NewPadded(text), compactTextTheme{Theme: theme.Current()})
	trailingIcon := container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(navigationRowTrailingIconSize), widget.NewIcon(trailing)))
	content := container.NewPadded(container.NewBorder(nil, nil, container.NewCenter(leading), trailingIcon, container.NewVBox(layout.NewSpacer(), compact, layout.NewSpacer())))
	button := widget.NewButton("", tapped)
	button.Importance = widget.LowImportance
	return container.NewStack(button, content)
}
