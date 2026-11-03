// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"go.lucor.dev/paw/internal/model"
)

func (a *app) makeSettingsView() fyne.CanvasObject {
	if a.state == nil {
		a.state = &model.AppState{Settings: model.NewDefaultSettings()}
	}
	view := NewSettingsView(a.state, a.storage, a.win)
	view.configureAutoLock = a.configureAutoLock
	view.openAbout = func() { a.showAboutPawView(view.View) }
	view.BackButton.OnTapped = func() { a.win.SetContent(a.main) }
	return view.View
}
