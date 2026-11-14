// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

const (
	pawColorNameFaviconBackground      fyne.ThemeColorName = "pawFaviconBackground"
	pawColorNameSuccessBadgeBackground fyne.ThemeColorName = "pawSuccessBadgeBackground"
	pawColorNameSuccessBadgeForeground fyne.ThemeColorName = "pawSuccessBadgeForeground"
	pawColorNameInfoBadgeBackground    fyne.ThemeColorName = "pawInfoBadgeBackground"
	pawColorNameInfoBadgeForeground    fyne.ThemeColorName = "pawInfoBadgeForeground"
	pawColorNameNeutralBadgeBackground fyne.ThemeColorName = "pawNeutralBadgeBackground"
	pawColorNameNeutralBadgeForeground fyne.ThemeColorName = "pawNeutralBadgeForeground"

	pawSizeNamePagePadding    fyne.ThemeSizeName = "pawPagePadding"
	pawSizeNameFormSpacing    fyne.ThemeSizeName = "pawFormSpacing"
	pawSizeNameListIcon       fyne.ThemeSizeName = "pawListIcon"
	pawSizeNameListIconFrame  fyne.ThemeSizeName = "pawListIconFrame"
	pawSizeNameListIconArea   fyne.ThemeSizeName = "pawListIconArea"
	pawSizeNameDetailIcon     fyne.ThemeSizeName = "pawDetailIcon"
	pawSizeNameDetailIconArea fyne.ThemeSizeName = "pawDetailIconArea"
)

// pawTheme preserves Fyne defaults and overrides Paw's shared semantic colors.
type pawTheme struct {
	fyne.Theme
	variant       fyne.ThemeVariant
	variantPinned bool
}

// newPawTheme returns Paw's minimal semantic color theme.
func newPawTheme() *pawTheme {
	return &pawTheme{Theme: theme.DefaultTheme()}
}

func (t *pawTheme) setVariant(variant fyne.ThemeVariant, pinned bool) {
	t.variant = variant
	t.variantPinned = pinned
}

// Color overrides Paw's semantic palette and delegates all other colors.
func (t *pawTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == pawColorNameFaviconBackground {
		return color.NRGBA{R: 0xF5, G: 0xF5, B: 0xF5, A: 0xFF}
	}
	if t.variantPinned {
		variant = t.variant
	}

	dark := variant == theme.VariantDark
	switch name {
	case pawColorNameSuccessBadgeBackground:
		if dark {
			return color.NRGBA{R: 0x17, G: 0x3A, B: 0x28, A: 0xFF}
		}
		return color.NRGBA{R: 0xDC, G: 0xFC, B: 0xE7, A: 0xFF}
	case pawColorNameSuccessBadgeForeground:
		if dark {
			return color.NRGBA{R: 0x86, G: 0xEF, B: 0xAC, A: 0xFF}
		}
		return color.NRGBA{R: 0x15, G: 0x80, B: 0x3D, A: 0xFF}
	case pawColorNameInfoBadgeBackground:
		if dark {
			return color.NRGBA{R: 0x1E, G: 0x35, B: 0x5B, A: 0xFF}
		}
		return color.NRGBA{R: 0xDB, G: 0xEA, B: 0xFE, A: 0xFF}
	case pawColorNameInfoBadgeForeground:
		if dark {
			return color.NRGBA{R: 0x93, G: 0xC5, B: 0xFD, A: 0xFF}
		}
		return color.NRGBA{R: 0x1D, G: 0x4E, B: 0xD8, A: 0xFF}
	case pawColorNameNeutralBadgeBackground:
		if dark {
			return color.NRGBA{R: 0x33, G: 0x37, B: 0x3F, A: 0xFF}
		}
		return color.NRGBA{R: 0xF3, G: 0xF4, B: 0xF6, A: 0xFF}
	case pawColorNameNeutralBadgeForeground:
		if dark {
			return color.NRGBA{R: 0xCB, G: 0xD0, B: 0xD8, A: 0xFF}
		}
		return color.NRGBA{R: 0x37, G: 0x41, B: 0x51, A: 0xFF}
	}
	switch name {
	case theme.ColorNameForeground:
		if dark {
			return color.NRGBA{R: 0xF1, G: 0xF2, B: 0xF4, A: 0xFF}
		}
		return color.NRGBA{R: 0x25, G: 0x28, B: 0x2E, A: 0xFF}
	case theme.ColorNameDisabled:
		if dark {
			return color.NRGBA{R: 0xA9, G: 0xAD, B: 0xB5, A: 0xFF}
		}
		return color.NRGBA{R: 0x68, G: 0x70, B: 0x7A, A: 0xFF}
	case theme.ColorNamePlaceHolder:
		if dark {
			return color.NRGBA{R: 0x92, G: 0x98, B: 0xA3, A: 0xFF}
		}
		return color.NRGBA{R: 0x73, G: 0x7A, B: 0x84, A: 0xFF}
	case theme.ColorNameInputBorder:
		if dark {
			return color.NRGBA{R: 0x76, G: 0x7D, B: 0x89, A: 0xFF}
		}
		return color.NRGBA{R: 0x85, G: 0x8B, B: 0x95, A: 0xFF}
	case theme.ColorNameDisabledButton:
		if dark {
			return color.NRGBA{R: 0x33, G: 0x36, B: 0x3D, A: 0xFF}
		}
		return color.NRGBA{R: 0xE6, G: 0xE9, B: 0xED, A: 0xFF}
	case theme.ColorNameError:
		if dark {
			return color.NRGBA{R: 0xFF, G: 0x85, B: 0x85, A: 0xFF}
		}
		return color.NRGBA{R: 0xB4, G: 0x23, B: 0x18, A: 0xFF}
	default:
		return t.Theme.Color(name, variant)
	}
}

// Size defines Paw's shared spacing and icon sizes from scalable Fyne values.
func (t *pawTheme) Size(name fyne.ThemeSizeName) float32 {
	padding := t.Theme.Size(theme.SizeNamePadding)
	switch name {
	case pawSizeNamePagePadding:
		return padding * 6
	case pawSizeNameFormSpacing:
		return padding * 4
	case pawSizeNameListIcon:
		return padding * 4
	case pawSizeNameListIconFrame:
		return padding * 8
	case pawSizeNameListIconArea:
		return padding * 10
	case pawSizeNameDetailIcon:
		return padding * 6
	case pawSizeNameDetailIconArea:
		return padding * 8
	default:
		return t.Theme.Size(name)
	}
}

// pawSize reads a Paw token from the active theme, with a fallback for unthemed test widgets.
func pawSize(name fyne.ThemeSizeName) float32 {
	if app := fyne.CurrentApp(); app != nil {
		if _, ok := app.Settings().Theme().(*pawTheme); ok {
			return theme.Size(name)
		}
	}
	return newPawTheme().Size(name)
}

// pawColor reads a Paw color from the active theme, with a fallback for unthemed test widgets.
func pawColor(name fyne.ThemeColorName) color.Color {
	if app := fyne.CurrentApp(); app != nil {
		if _, ok := app.Settings().Theme().(*pawTheme); ok {
			return theme.Color(name)
		}
		return newPawTheme().Color(name, app.Settings().ThemeVariant())
	}
	return newPawTheme().Color(name, theme.VariantLight)
}

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
	titleStyle        = textStyle{size: theme.SizeNameSubHeadingText, weight: fyne.TextStyle{Bold: true}}
	itemTitleStyle    = textStyle{size: theme.SizeNameText, weight: fyne.TextStyle{Bold: true}}
	formLabelStyle    = itemTitleStyle
	sectionTitleStyle = textStyle{size: theme.SizeNameCaptionText, weight: fyne.TextStyle{Bold: true}}
	countStyle        = sectionTitleStyle
	bodyStyle         = textStyle{size: theme.SizeNameText, wrapping: fyne.TextWrapWord}
	captionStyle      = textStyle{size: theme.SizeNameCaptionText, wrapping: fyne.TextWrapWord}
	headingStyle      = textStyle{size: theme.SizeNameHeadingText, weight: fyne.TextStyle{Bold: true}}
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
