// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package test

import (
	"testing"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
)

// NewApp creates a Fyne test app.
func NewApp() fyne.App {
	return fynetest.NewApp()
}

// NewTempApp creates a Fyne test app that is closed when the test ends.
func NewTempApp(t testing.TB) fyne.App {
	return fynetest.NewTempApp(t)
}

// NewTempWindow creates a Fyne test window that is closed when the test ends.
func NewTempWindow(t testing.TB, content fyne.CanvasObject) fyne.Window {
	return fynetest.NewTempWindow(t, content)
}

// LaidOutObjects returns visible objects laid out below object.
func LaidOutObjects(object fyne.CanvasObject) []fyne.CanvasObject {
	return fynetest.LaidOutObjects(object)
}

// Tap simulates a tap on a Fyne object.
func Tap(object fyne.Tappable) {
	fynetest.Tap(object)
}
