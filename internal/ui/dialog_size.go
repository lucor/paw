// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

const (
	compactDialogMaxWidth   = 360
	compactDialogSideMargin = 16
)

// sizeCompactDialog limits a focused dialog to the available window width.
func sizeCompactDialog(d dialog.Dialog, window fyne.Window) {
	d.Resize(compactDialogSize(d.MinSize(), window.Canvas().Size()))
}

func compactDialogSize(minimum, windowSize fyne.Size) fyne.Size {
	availableWidth := windowSize.Width - 2*compactDialogSideMargin
	width := float32(math.Min(compactDialogMaxWidth, float64(availableWidth)))
	if width < minimum.Width {
		width = minimum.Width
	}
	return fyne.NewSize(width, minimum.Height)
}
