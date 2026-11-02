// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// configureFilePicker applies Paw's shared desktop size and list presentation.
func configureFilePicker(picker *dialog.FileDialog) {
	picker.SetView(dialog.ListView)
	if !fyne.CurrentDevice().IsMobile() {
		picker.Resize(fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight))
	}
}
