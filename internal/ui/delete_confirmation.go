// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const (
	deleteConfirmationIconSize = 20
	deleteConfirmationMaxWidth = 400
)

// showDeleteConfirmation displays a text-only destructive confirmation.
func showDeleteConfirmation(titleText, actionText, messageText string, confirm func(), parent fyne.Window) *dialog.CustomDialog {
	titleIcon := container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(deleteConfirmationIconSize), widget.NewIcon(icons.Delete)))
	title := newText(titleText, itemTitleStyle)
	message := newText(messageText, bodyStyle)
	content := container.NewVBox(container.NewHBox(titleIcon, title), message)
	d := dialog.NewCustom("", "", content, parent)

	escape := &desktop.CustomShortcut{KeyName: fyne.KeyEscape}
	removeEscape := func() { parent.Canvas().RemoveShortcut(escape) }
	cancel := widget.NewButton("Cancel", func() {
		removeEscape()
		d.Hide()
	})
	cancel.Importance = widget.LowImportance
	deleteButton := widget.NewButton(actionText, func() {
		removeEscape()
		d.Hide()
		confirm()
	})
	deleteButton.Importance = widget.DangerImportance
	d.SetButtons([]fyne.CanvasObject{container.NewHBox(layout.NewSpacer(), cancel, deleteButton)})
	dialogSize := d.MinSize()
	availableWidth := parent.Canvas().Size().Width - 2*theme.Size(theme.SizeNamePadding)
	if availableWidth > 0 {
		if availableWidth > deleteConfirmationMaxWidth {
			availableWidth = deleteConfirmationMaxWidth
		}
		if availableWidth > dialogSize.Width {
			dialogSize.Width = availableWidth
			d.Resize(dialogSize)
		}
	}
	parent.Canvas().AddShortcut(escape, func(fyne.Shortcut) {
		removeEscape()
		d.Hide()
	})
	d.Show()
	return d
}
