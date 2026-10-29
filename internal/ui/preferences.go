// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"go.lucor.dev/paw/internal/paw"
)

func (a *app) makePreferencesView() fyne.CanvasObject {
	if a.state == nil {
		a.state = &paw.AppState{Preferences: paw.NewDefaultPreferences()}
	}
	view := NewPreferencesView(a.state, a.storage, a.win)
	view.BackButton.OnTapped = func() { a.win.SetContent(a.main) }
	return view.View
}
