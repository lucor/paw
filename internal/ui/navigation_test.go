// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// TestNavigationLayoutAndSelection checks equal cells, icon sizing and native input.
func TestNavigationLayoutAndSelection(t *testing.T) {
	app := test.NewTempApp(t)
	nav := NewNavigation(NavigationItem{icons.Vault, icons.VaultActive}, NavigationItem{icons.Codes, icons.CodesActive}, NavigationItem{icons.Settings, icons.SettingsActive})
	window := test.NewTempWindow(t, nav)
	var selections []int
	nav.OnSelected = func(index int) { selections = append(selections, index) }
	for _, width := range []float32{360, 800} {
		window.Resize(fyne.NewSize(width, 64))
		for i, button := range nav.buttons {
			require.InDelta(t, nav.buttons[0].Size().Width, button.Size().Width, 0.01)
			require.GreaterOrEqual(t, button.button.Size().Height, float32(48))
			renderer := test.WidgetRenderer(button).(*navigationButtonRenderer)
			require.Equal(t, fyne.NewSquareSize(24), renderer.icon.Size())
			position := app.Driver().AbsolutePositionForObject(renderer.icon).Add(fyne.NewPos(12, 12))
			test.TapCanvas(window.Canvas(), position)
			require.Equal(t, i, nav.Selected)
			require.Equal(t, button.item.SelectedIcon, renderer.icon.Resource)
		}
	}
	nav.buttons[1].button.TypedKey(&fyne.KeyEvent{Name: fyne.KeySpace})
	require.Equal(t, 1, nav.Selected)
	count := len(selections)
	nav.Select(1)
	require.Len(t, selections, count, "reselecting the same destination must not trigger navigation")
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
		nav.Refresh()
		renderer := test.WidgetRenderer(nav.buttons[1]).(*navigationButtonRenderer)
		require.Equal(t, theme.Color(theme.ColorNamePrimary), renderer.indicator.FillColor)
	}
}
