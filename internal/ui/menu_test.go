// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func TestMainMenuAboutOpensAboutPawView(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow(AppTitle)
	t.Cleanup(window.Close)
	main := container.NewVScroll(widget.NewLabel("Vault list"))
	window.SetContent(main)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]storage.VaultInfo, error) { return nil, nil }
	a := &app{win: window, main: main, storage: store}

	menu := a.makeMainMenu()
	var about, quit *fyne.MenuItem
	for _, section := range menu.Items {
		for _, item := range section.Items {
			if item.Label == menuAboutLabel {
				about = item
			}
			if item.Label == menuQuitLabel {
				quit = item
			}
		}
	}
	require.NotNil(t, about)
	require.NotNil(t, quit)
	require.Equal(t, menuQuitLabel, quit.Label)

	about.Action()
	require.Contains(t, auditTexts(window.Content()), aboutPageTitle)
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Back" {
			test.Tap(button)
			break
		}
	}
	require.Same(t, main, window.Content())
}
