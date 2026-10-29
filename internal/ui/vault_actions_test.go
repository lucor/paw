// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
)

func TestVaultActionsGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			view := NewEntryList(entryListFixture())
			window := test.NewTempWindow(t, view)
			window.Resize(fyne.NewSize(320, 540))
			for _, object := range test.LaidOutObjects(view) {
				if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Vault actions" {
					test.Tap(button)
				}
			}
			test.AssertImageMatches(t, "vault-actions-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}

func TestVaultSelectorMenuGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			view := NewEntryList(entryListFixture())
			window := test.NewTempWindow(t, view)
			window.Resize(fyne.NewSize(320, 540))
			for _, object := range test.LaidOutObjects(view) {
				if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Select vault" {
					test.Tap(button)
				}
			}
			test.AssertImageMatches(t, "vault-selector-menu-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}

func TestVaultHeaderMenus(t *testing.T) {
	app := test.NewTempApp(t)
	options := entryListFixture()
	options.VaultName = "A vault with a long display name"
	called := ""
	options.VaultActions = NewVaultActions(func() { called = "audit" }, func() { called = "export" }, func() { called = "lock" })
	view := NewEntryList(options)
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(320, 540))
	buttons := map[string]*iconButton{}
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok {
			buttons[button.AccessibilityLabel()] = button
			position := app.Driver().AbsolutePositionForObject(button)
			require.LessOrEqual(t, position.X+button.Size().Width, window.Canvas().Size().Width)
		}
	}
	for _, name := range []string{"Select vault", "Add entry", "Vault actions"} {
		require.Contains(t, buttons, name)
	}
	test.Tap(buttons["Select vault"])
	labels := menuLabels(t, window)
	require.Equal(t, []string{"Personal — Unlocked", "Work — Locked", "Add vault"}, labels)
	// Tapping outside dismisses the selector before opening another menu.
	test.TapCanvas(window.Canvas(), fyne.NewPos(300, 500))
	for index, name := range []string{"Password audit", "Export credentials", "Lock vault"} {
		test.Tap(buttons["Vault actions"])
		require.Equal(t, []string{"Password audit", "Export credentials", "Lock vault"}, menuLabels(t, window))
		menu := window.Canvas().Overlays().Top()
		for _, object := range test.LaidOutObjects(menu) {
			control, ok := object.(fyne.Widget)
			if !ok {
				continue
			}
			for _, rendered := range test.WidgetRenderer(control).Objects() {
				if text, ok := rendered.(*canvas.Text); ok && text.Text == name {
					test.Tap(object.(fyne.Tappable))
				}
			}
		}
		require.Equal(t, []string{"audit", "export", "lock"}[index], called)
		require.Nil(t, window.Canvas().Overlays().Top())
	}
	for _, item := range options.VaultActions {
		if !item.IsSeparator {
			require.NotNil(t, item.Icon)
			require.False(t, item.Disabled)
		}
	}
}

func menuLabels(t *testing.T, window fyne.Window) []string {
	t.Helper()
	menu := window.Canvas().Overlays().Top()
	require.NotNil(t, menu)
	var labels []string
	for _, object := range test.LaidOutObjects(menu) {
		if popup, ok := object.(*widget.PopUpMenu); ok {
			position := fyne.CurrentApp().Driver().AbsolutePositionForObject(popup)
			require.GreaterOrEqual(t, position.X, float32(0))
			require.LessOrEqual(t, position.X+popup.Size().Width, window.Canvas().Size().Width)
		}
		if text, ok := object.(*canvas.Text); ok && text.Text != "" {
			labels = append(labels, text.Text)
		}
	}
	return labels
}
