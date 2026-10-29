// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

var passwordStrengthLabels = [...]string{"Very weak", "Weak", "Fair", "Strong", "Very strong"}

// PasswordStrengthLabel returns the display label for a zxcvbn score.
func PasswordStrengthLabel(score int) string {
	if score < 0 {
		score = 0
	}
	if score > 4 {
		score = 4
	}
	return passwordStrengthLabels[score]
}

// PasswordStrengthIndicator displays a five-step password strength estimate.
type PasswordStrengthIndicator struct {
	widget.BaseWidget
	segments []*canvas.Rectangle
	label    *widget.Label
	score    int
	set      bool
}

// NewPasswordStrengthIndicator creates a hidden indicator.
func NewPasswordStrengthIndicator() *PasswordStrengthIndicator {
	i := &PasswordStrengthIndicator{label: newText("", helpStyle)}
	for range 5 {
		segment := canvas.NewRectangle(color.Transparent)
		segment.SetMinSize(fyne.NewSize(20, 6))
		segment.CornerRadius = 3
		i.segments = append(i.segments, segment)
	}
	i.label.Wrapping = fyne.TextWrapOff
	i.ExtendBaseWidget(i)
	i.Hide()
	return i
}

// SetScore updates the indicator for a zxcvbn score from zero to four.
func (i *PasswordStrengthIndicator) SetScore(score int) {
	if score < 0 {
		score = 0
	}
	if score > 4 {
		score = 4
	}
	if i.set && i.score == score && i.Visible() {
		return
	}
	i.score, i.set = score, true
	i.label.SetText(PasswordStrengthLabel(score))
	i.Show()
	i.Refresh()
}

// Clear hides the indicator when the password field is empty or reset.
func (i *PasswordStrengthIndicator) Clear() {
	i.set = false
	i.label.SetText("")
	i.Hide()
}

// CreateRenderer builds the segments and label using native Fyne objects.
func (i *PasswordStrengthIndicator) CreateRenderer() fyne.WidgetRenderer {
	segments := make([]fyne.CanvasObject, len(i.segments))
	for index, segment := range i.segments {
		segments[index] = segment
	}
	bar := container.New(layout.NewGridLayout(5), segments...)
	centeredBar := container.NewVBox(layout.NewSpacer(), bar, layout.NewSpacer())
	r := &passwordStrengthRenderer{WidgetRenderer: widget.NewSimpleRenderer(container.NewGridWithColumns(2, centeredBar, i.label)), indicator: i}
	r.Refresh()
	return r
}

type passwordStrengthRenderer struct {
	fyne.WidgetRenderer
	indicator *PasswordStrengthIndicator
}

// Refresh resolves colors again when the application's theme changes.
func (r *passwordStrengthRenderer) Refresh() {
	i := r.indicator
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	inactive := i.Theme().Color(theme.ColorNameInputBorder, variant)
	for index, segment := range i.segments {
		segment.FillColor = inactive
		if i.set && index <= i.score {
			segment.FillColor = passwordStrengthColor(i.score, variant)
		}
		segment.Refresh()
	}
	r.WidgetRenderer.Refresh()
}
