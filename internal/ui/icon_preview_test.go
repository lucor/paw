// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestFaviconPreviewBackgroundStaysLightAcrossThemes(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			favicon := darkFaviconFixture()
			preview := newIconPreview(favicon)
			renderer := preview.CreateRenderer()
			renderer.Refresh()

			require.Equal(t, faviconPreviewBackground, preview.background.FillColor)
			require.Equal(t, favicon.Data, preview.icon.Resource.Content())

			preview.SetResource(icons.Globe)
			require.Equal(t, theme.Color(theme.ColorNameInputBackground), preview.background.FillColor)
			require.NotEqual(t, faviconPreviewBackground, preview.background.FillColor)
			preview.SetFavicon(favicon)
			require.Equal(t, faviconPreviewBackground, preview.background.FillColor)

			other := theme.VariantDark
			if variant == theme.VariantDark {
				other = theme.VariantLight
			}
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: other})
			renderer.Refresh()
			require.Equal(t, faviconPreviewBackground, preview.background.FillColor)
		})
	}
}

func TestFaviconPreviewIsUsedInVaultListAndActionRows(t *testing.T) {
	test.NewTempApp(t)
	favicon := darkFaviconFixture()
	options := entryListFixture()
	options.Entries[0].Icon = favicon
	list := NewEntryList(options)
	test.NewTempWindow(t, list)
	listPreviews := 0
	for _, object := range test.LaidOutObjects(list) {
		if preview, ok := object.(*iconPreview); ok {
			listPreviews++
			preview.CreateRenderer().Refresh()
			require.Equal(t, faviconPreviewBackground, preview.background.FillColor)
		}
	}
	require.Positive(t, listPreviews)

	row := NewActionRow("GitHub", "dev@example.com", favicon, func() {})
	row.SetFlat(true)
	test.NewTempWindow(t, row)
	var codesPreview *iconPreview
	for _, object := range test.LaidOutObjects(row) {
		if preview, ok := object.(*iconPreview); ok {
			codesPreview = preview
			preview.CreateRenderer().Refresh()
			break
		}
	}
	require.NotNil(t, codesPreview)
	require.Equal(t, faviconPreviewBackground, codesPreview.background.FillColor)
}

func TestEntryDetailUsesFaviconPreview(t *testing.T) {
	test.NewTempApp(t)
	item := detailFixture(t)
	item.Metadata.Favicon = darkFaviconFixture()
	window := test.NewTempWindow(t, nil)
	view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	var preview *iconPreview
	for _, object := range test.LaidOutObjects(view) {
		if candidate, ok := object.(*iconPreview); ok {
			preview = candidate
			preview.CreateRenderer().Refresh()
			break
		}
	}
	require.NotNil(t, preview)
	require.Equal(t, faviconPreviewBackground, preview.background.FillColor)
}

func darkFaviconFixture() *paw.Favicon {
	const size = 32
	imageData := image.NewNRGBA(image.Rect(0, 0, size, size))
	black := color.NRGBA{A: 255}
	for y := 7; y < 26; y++ {
		for x := 7; x < 25; x++ {
			dx, dy := x-16, y-16
			if dx*dx+dy*dy <= 90 {
				imageData.SetNRGBA(x, y, black)
			}
		}
	}
	for y := 3; y < 12; y++ {
		for x := 8; x < 14; x++ {
			imageData.SetNRGBA(x, y, black)
		}
		for x := 19; x < 25; x++ {
			imageData.SetNRGBA(x, y, black)
		}
	}
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, imageData)
	return paw.NewFavicon("github.com", encoded.Bytes(), "png")
}
