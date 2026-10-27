// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// textStyle defines one typography role using native Fyne theme values.
type textStyle struct {
	size       fyne.ThemeSizeName
	weight     fyne.TextStyle
	importance widget.Importance
	wrapping   fyne.TextWrap
}

var (
	viewTitleStyle      = textStyle{size: theme.SizeNameText, weight: fyne.TextStyle{Bold: true}}
	headingStyle        = textStyle{size: theme.SizeNameHeadingText, weight: fyne.TextStyle{Bold: true}}
	subtitleStyle       = textStyle{size: theme.SizeNameText, wrapping: fyne.TextWrapWord}
	formLabelStyle      = textStyle{size: theme.SizeNameText}
	helpStyle           = textStyle{size: theme.SizeNameCaptionText, wrapping: fyne.TextWrapWord}
	rowTitleStyle       = textStyle{size: theme.SizeNameText, weight: fyne.TextStyle{Bold: true}}
	rowDescriptionStyle = textStyle{size: theme.SizeNameText, wrapping: fyne.TextWrapWord}
	bodyStyle           = textStyle{size: theme.SizeNameText, wrapping: fyne.TextWrapWord}
	errorStyle          = textStyle{size: theme.SizeNameText, importance: widget.DangerImportance, wrapping: fyne.TextWrapWord}
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

// compactTextTheme removes label insets in compact list rows.
type compactTextTheme struct {
	fyne.Theme
}

// Size removes inner padding while preserving the rest of the active theme.
func (t compactTextTheme) Size(name fyne.ThemeSizeName) float32 {
	if name == theme.SizeNameInnerPadding {
		return 0
	}
	return t.Theme.Size(name)
}
