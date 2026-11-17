// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetest "fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
)

func TestSecondaryPageAlignsHeaderBodyAndFooter(t *testing.T) {
	app := fynetest.NewTempApp(t)
	header := canvas.NewText("Header", nil)
	body := canvas.NewText("Body", nil)
	footer := canvas.NewText("Footer", nil)
	page := newSecondaryPage(header, footer, body)
	window := fynetest.NewTempWindow(t, page)
	window.Resize(fyne.NewSize(440, 650))
	origin := app.Driver().AbsolutePositionForObject(page)
	for _, part := range []fyne.CanvasObject{header, body, footer} {
		require.InDelta(t, origin.X, app.Driver().AbsolutePositionForObject(part).X, 0.01)
		require.InDelta(t, page.Size().Width, part.Size().Width, 0.01)
	}
	require.InDelta(t, origin.Y, app.Driver().AbsolutePositionForObject(header).Y, 0.01)
	require.InDelta(t, origin.Y+page.Size().Height, app.Driver().AbsolutePositionForObject(footer).Y+footer.Size().Height, 0.01)
}

func TestMainSectionsUseFullPageWidth(t *testing.T) {
	app := fynetest.NewTempApp(t)
	window := fynetest.NewTempWindow(t, nil)
	codes := newCodesView(context.Background(), nil, nil)
	settings := NewSettingsView(nil, nil, window)
	settings.BackButton.Hide()
	options := entryListFixture()
	options.Codes = codes.View
	options.Settings = settings.section
	view := NewEntryList(options)
	window.SetContent(view)

	var navigation *Navigation
	for _, object := range fynetest.LaidOutObjects(view) {
		if candidate, ok := object.(*Navigation); ok {
			navigation = candidate
		}
	}
	require.NotNil(t, navigation)

	for _, width := range []float32{320, 600} {
		window.Resize(fyne.NewSize(width, 650))
		navigation.Select(0)
		search := findEntryListEntry(t, view)
		vaultX := app.Driver().AbsolutePositionForObject(search).X
		vaultWidth := search.Size().Width
		require.InDelta(t, app.Driver().AbsolutePositionForObject(view).X, vaultX, 0.01)
		require.InDelta(t, view.Size().Width, vaultWidth, 0.01)

		for index, section := range []fyne.CanvasObject{codes.View, settings.section} {
			navigation.Select(index + 1)
			require.InDelta(t, vaultX, app.Driver().AbsolutePositionForObject(section).X, 0.01)
			require.InDelta(t, vaultWidth, section.Size().Width, 0.01)
		}
	}
}
