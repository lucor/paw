// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package widgets contains small custom Fyne widgets used by the Paw UI.
package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// Badge displays short text inside a rounded colored background.
type Badge struct {
	widget.BaseWidget

	Text       string
	Background color.Color
	Foreground color.Color

	Radius   float32
	TextSize float32
	PaddingX float32
	PaddingY float32
	Action   *widget.Button
}

// NewBadge creates a badge with comfortable defaults for compact status text.
func NewBadge(text string, background, foreground color.Color) *Badge {
	badge := &Badge{
		Text:       text,
		Background: background,
		Foreground: foreground,
		Radius:     theme.Size(theme.SizeNameCardRadius) + 2,
		TextSize:   12,
		PaddingX:   10,
		PaddingY:   4,
	}
	badge.ExtendBaseWidget(badge)
	return badge
}

// SetText updates the badge text.
func (b *Badge) SetText(text string) {
	if b.Text == text {
		return
	}
	b.Text = text
	b.Refresh()
}

// SetColors updates the badge background and text colors.
func (b *Badge) SetColors(background, foreground color.Color) {
	b.Background = background
	b.Foreground = foreground
	b.Refresh()
}

// SetRadius updates the badge corner radius.
func (b *Badge) SetRadius(radius float32) {
	b.Radius = radius
	b.Refresh()
}

// SetAction adds a compact trailing action inside the badge.
func (b *Badge) SetAction(tapped func()) {
	b.Action = widget.NewButton("×", tapped)
	b.Action.Importance = widget.LowImportance
	b.Refresh()
}

// CreateRenderer creates the badge background and centered text.
func (b *Badge) CreateRenderer() fyne.WidgetRenderer {
	background := canvas.NewRectangle(b.Background)
	background.CornerRadius = b.Radius
	label := canvas.NewText(b.Text, b.Foreground)
	label.TextSize = b.TextSize
	label.Alignment = fyne.TextAlignCenter
	return &badgeRenderer{badge: b, background: background, label: label, action: b.Action}
}

// badgeRenderer lays out the badge text over its rounded background.
type badgeRenderer struct {
	badge      *Badge
	background *canvas.Rectangle
	label      *canvas.Text
	action     *widget.Button
}

// Objects returns the canvas objects drawn by the badge.
func (r *badgeRenderer) Objects() []fyne.CanvasObject {
	objects := []fyne.CanvasObject{r.background, r.label}
	if r.action != nil {
		objects = append(objects, r.action)
	}
	return objects
}

// MinSize returns the text size plus the badge's horizontal and vertical padding.
func (r *badgeRenderer) MinSize() fyne.Size {
	textSize := r.label.MinSize()
	width := textSize.Width + r.badge.PaddingX*2
	height := textSize.Height + r.badge.PaddingY*2
	if r.action != nil {
		actionSize := r.action.MinSize()
		width += actionSize.Width
		if actionSize.Height > height {
			height = actionSize.Height
		}
	}
	return fyne.NewSize(width, height)
}

// Layout sizes the background and centers the text within it.
func (r *badgeRenderer) Layout(size fyne.Size) {
	badgeSize := r.MinSize()
	if badgeSize.Width > size.Width {
		badgeSize.Width = size.Width
	}
	if badgeSize.Height > size.Height {
		badgeSize.Height = size.Height
	}
	r.background.Resize(badgeSize)
	r.background.Move(fyne.NewPos(
		(size.Width-badgeSize.Width)/2,
		(size.Height-badgeSize.Height)/2,
	))
	textSize := r.label.MinSize()
	x := r.badge.PaddingX
	r.label.Resize(textSize)
	r.label.Move(fyne.NewPos(x, (size.Height-textSize.Height)/2))
	if r.action != nil {
		actionSize := r.action.MinSize()
		r.action.Move(fyne.NewPos(size.Width-actionSize.Width, (size.Height-actionSize.Height)/2))
		r.action.Resize(actionSize)
	}
}

// Refresh updates the badge canvas objects from its current properties.
func (r *badgeRenderer) Refresh() {
	r.background.FillColor = r.badge.Background
	r.background.CornerRadius = r.badge.Radius
	r.label.Text = r.badge.Text
	r.label.Color = r.badge.Foreground
	r.label.TextSize = r.badge.TextSize
	r.action = r.badge.Action
	r.label.Refresh()
	r.background.Refresh()
	r.Layout(r.badge.Size())
}

// Destroy releases renderer resources.
func (r *badgeRenderer) Destroy() {}
