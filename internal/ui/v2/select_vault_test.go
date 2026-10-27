// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
)

func TestVaultSelectorGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			assertViewGolden(t, "vault-selector-"+variantName(variant)+".png", variant, fyne.NewSize(440, 650), selectorPreview)
		})
	}
}

func TestVaultSelectorInteractions(t *testing.T) {
	test.NewTempApp(t)
	opened := ""
	window := test.NewTempWindow(t, NewSelectVault([]VaultChoice{
		{Name: "Personal", Open: func() { opened = "Personal" }},
		{Name: "Work", Open: func() { opened = "Work" }},
		{Name: "Archive", Unlocked: true, Open: func() { opened = "Archive" }},
	}, func() {}))
	window.Resize(fyne.NewSize(440, 650))
	for _, object := range test.LaidOutObjects(window.Content()) {
		if list, ok := object.(*widget.List); ok {
			list.Select(1)
			require.Equal(t, "Work", opened)
			list.Select(2)
			require.Equal(t, "Archive", opened)
			return
		}
	}
	t.Fatal("vault list was not rendered")
}

func selectorPreview() fyne.CanvasObject {
	return NewSelectVault([]VaultChoice{
		{Name: "Personal", Open: func() {}},
		{Name: "Work", Open: func() {}},
		{Name: "Archive", Unlocked: true, Open: func() {}},
	}, func() {})
}

func assertViewGolden(t *testing.T, golden string, variant fyne.ThemeVariant, size fyne.Size, view func() fyne.CanvasObject) {
	t.Helper()
	app := test.NewTempApp(t)
	app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
	window := test.NewTempWindow(t, view())
	window.Resize(size)
	test.AssertImageMatches(t, golden, window.Canvas().Capture())
}

func variantName(variant fyne.ThemeVariant) string {
	if variant == theme.VariantDark {
		return "dark"
	}
	return "light"
}
