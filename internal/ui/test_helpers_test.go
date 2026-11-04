// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

type variantTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t variantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return (&pawTheme{Theme: t.Theme, variant: t.variant, variantPinned: true}).Color(name, t.variant)
}

func (t variantTheme) Size(name fyne.ThemeSizeName) float32 {
	return (&pawTheme{Theme: t.Theme}).Size(name)
}

func findIcon(t *testing.T, objects []fyne.CanvasObject, name string) *widget.Icon {
	t.Helper()
	for _, object := range objects {
		if icon, ok := object.(*widget.Icon); ok && icon.Resource.Name() == name {
			return icon
		}
	}
	t.Fatalf("icon %q was not rendered", name)
	return nil
}

var _ fyne.Theme = variantTheme{}
