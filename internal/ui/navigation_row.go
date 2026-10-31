// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// newNavigationRow creates a clickable row with one text line.
func newNavigationRow(title string, leading fyne.CanvasObject, tapped func()) fyne.CanvasObject {
	return newNavigationRowWithSubtitle(title, "", leading, tapped)
}

// newNavigationRowWithSubtitle adds an optional secondary line to the same layout.
func newNavigationRowWithSubtitle(title, subtitle string, leading fyne.CanvasObject, tapped func()) fyne.CanvasObject {
	label := newText(title, bodyStyle)
	label.TextStyle.Bold = true
	text := container.NewVBox(label)
	if subtitle != "" {
		text.Add(newText(subtitle, captionStyle))
	}
	compact := container.NewThemeOverride(container.NewPadded(text), compactTextTheme{Theme: theme.Current()})
	chevron := container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(16), widget.NewIcon(icons.Next)))
	content := container.NewPadded(container.NewBorder(nil, nil, container.NewCenter(leading), chevron, container.NewVBox(layout.NewSpacer(), compact, layout.NewSpacer())))
	button := widget.NewButton("", tapped)
	button.Importance = widget.LowImportance
	return container.NewStack(button, content)
}
