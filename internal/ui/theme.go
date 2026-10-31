// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// passwordStrengthColor returns the fixed color for a score in the active theme.
func passwordStrengthColor(score int, variant fyne.ThemeVariant) color.Color {
	light := [...]color.NRGBA{{R: 198, G: 40, B: 40, A: 255}, {R: 217, G: 95, B: 2, A: 255}, {R: 184, G: 134, B: 0, A: 255}, {R: 56, G: 142, B: 60, A: 255}, {R: 27, G: 94, B: 32, A: 255}}
	dark := [...]color.NRGBA{{R: 239, G: 83, B: 80, A: 255}, {R: 255, G: 167, B: 38, A: 255}, {R: 255, G: 213, B: 79, A: 255}, {R: 129, G: 199, B: 132, A: 255}, {R: 76, G: 175, B: 80, A: 255}}
	if variant != theme.VariantLight {
		return dark[score]
	}
	return light[score]
}

// textStyle defines one typography role using native Fyne theme values.
type textStyle struct {
	size       fyne.ThemeSizeName
	weight     fyne.TextStyle
	importance widget.Importance
	wrapping   fyne.TextWrap
}

var (
	titleStyle   = textStyle{size: theme.SizeNameSubHeadingText, weight: fyne.TextStyle{Bold: true}}
	bodyStyle    = textStyle{size: theme.SizeNameText, wrapping: fyne.TextWrapWord}
	captionStyle = textStyle{size: theme.SizeNameCaptionText, wrapping: fyne.TextWrapWord}
	headingStyle = textStyle{size: theme.SizeNameHeadingText, weight: fyne.TextStyle{Bold: true}}
)

// newText applies a shared typography role to a native label.
func newText(text string, style textStyle) *widget.Label {
	label := widget.NewLabel(text)
	label.SizeName = style.size
	label.TextStyle = style.weight
	label.Importance = style.importance
	label.Wrapping = style.wrapping
	return label
}

// newErrorText uses the Caption role with Fyne's semantic error color.
func newErrorText(text string) *widget.Label {
	label := newText(text, captionStyle)
	label.Importance = widget.DangerImportance
	return label
}

// compactTextTheme removes label insets in compact list rows.
type compactTextTheme struct {
	fyne.Theme
}

// Color follows the application theme when its selected variant changes.
func (t compactTextTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	return fyne.CurrentApp().Settings().Theme().Color(name, variant)
}

// Size removes inner padding while preserving the rest of the active theme.
func (t compactTextTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 0
	}
	return t.Theme.Size(name)
}
