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
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// ActionRow is a full-width outlined action with a leading icon and two text lines.
type ActionRow struct {
	widget.BaseWidget
	button  *widget.Button
	leading *widget.Icon
	title   *widget.Label
	detail  *widget.Label
	content *fyne.Container
}

// NewActionRow creates a reusable, clickable row for navigation and list items.
func NewActionRow(title, detail string, resource fyne.Resource, tapped func()) *ActionRow {
	row := &ActionRow{button: widget.NewButton("", tapped)}
	row.button.Importance = widget.LowImportance
	row.leading = widget.NewIcon(resource)
	row.title = newText(title, rowTitleStyle)
	row.detail = newText(detail, rowDescriptionStyle)
	text := container.NewThemeOverride(container.New(layout.NewCustomPaddedVBoxLayout(0), row.title, row.detail), compactTextTheme{Theme: theme.Current()})
	icon := container.NewPadded(container.NewGridWrap(fyne.NewSquareSize(32), row.leading))
	row.content = container.NewBorder(nil, nil,
		container.NewCenter(icon),
		container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(theme.Size(theme.SizeNameInlineIcon)), widget.NewIcon(icons.Next))),
		text,
	)
	row.ExtendBaseWidget(row)
	return row
}

// SetContent updates the displayed data when a list reuses this row.
func (r *ActionRow) SetContent(title, detail string, resource fyne.Resource) {
	r.title.SetText(title)
	r.detail.SetText(detail)
	r.leading.SetResource(resource)
	r.Refresh()
}

// SetTapped changes the action when a list recycles this row for another item.
func (r *ActionRow) SetTapped(tapped func()) {
	r.button.OnTapped = tapped
}

// CreateRenderer keeps the outline above the button's native hover background.
func (r *ActionRow) CreateRenderer() fyne.WidgetRenderer {
	outline := canvas.NewRectangle(color.Transparent)
	padded := container.NewPadded(r.content)
	stack := container.NewStack(r.button, container.NewStack(outline, padded))
	renderer := &actionRowRenderer{
		WidgetRenderer: widget.NewSimpleRenderer(stack),
		row:            r,
		outline:        outline,
		padded:         padded,
	}
	renderer.Refresh()
	return renderer
}

type actionRowRenderer struct {
	fyne.WidgetRenderer
	row     *ActionRow
	outline *canvas.Rectangle
	padded  *fyne.Container
}

// Refresh updates the frame and inset when the active theme changes.
func (r *actionRowRenderer) Refresh() {
	th := r.row.Theme()
	variant := fyne.CurrentApp().Settings().ThemeVariant()
	r.outline.StrokeColor = th.Color(theme.ColorNameInputBorder, variant)
	r.outline.StrokeWidth = th.Size(theme.SizeNameInputBorder)
	r.outline.CornerRadius = th.Size(theme.SizeNameButtonRadius)
	inset := th.Size(theme.SizeNameInnerPadding)
	r.padded.Layout = layout.NewCustomPaddedLayout(inset, inset, inset, inset)
	r.WidgetRenderer.Refresh()
}
