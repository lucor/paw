// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// ActionRow is a full-width outlined action with a leading icon and two text lines.
type ActionRow struct {
	widget.BaseWidget
	button        *widget.Button
	leading       *widget.Icon
	leadingArea   *fyne.Container
	title         *widget.Label
	detail        *widget.Label
	content       *fyne.Container
	text          *fyne.Container
	trailing      *fyne.Container
	count         *widget.Label
	next          *widget.Icon
	supplemental  fyne.CanvasObject
	customLeading bool
	flat          bool
	interactive   bool
}

// NewActionRow creates a reusable, clickable row for navigation and list items.
func NewActionRow(title, detail string, resource fyne.Resource, tapped func()) *ActionRow {
	row := &ActionRow{button: widget.NewButton("", tapped), interactive: true}
	row.button.Importance = widget.LowImportance
	row.leading = widget.NewIcon(resource)
	row.title = newText(title, titleStyle)
	row.detail = newText(detail, bodyStyle)
	row.text = container.New(layout.NewCustomPaddedVBoxLayout(0), row.title, row.detail)
	text := container.NewThemeOverride(row.text, compactTextTheme{Theme: theme.Current()})
	row.leadingArea = container.NewPadded(container.NewGridWrap(fyne.NewSquareSize(32), row.leading))
	if favicon, ok := resource.(*model.Favicon); ok && len(favicon.Data) > 0 {
		row.leadingArea.Objects[0] = container.NewGridWrap(fyne.NewSquareSize(32), newIconPreview(favicon))
		row.customLeading = true
	}
	row.count = newText("", captionStyle)
	row.count.Wrapping = fyne.TextWrapOff
	row.count.Hide()
	row.next = widget.NewIcon(icons.Next)
	row.trailing = row.trailingArrow()
	row.content = container.NewBorder(nil, nil,
		container.NewCenter(row.leadingArea),
		row.trailing,
		text,
	)
	row.ExtendBaseWidget(row)
	return row
}

// SetLeadingObject replaces the standard icon with a custom object in the same row slot.
func (r *ActionRow) SetLeadingObject(object fyne.CanvasObject) {
	size := object.MinSize()
	r.leadingArea.Objects[0] = container.NewGridWrap(fyne.NewSize(size.Width, size.Height), object)
	r.customLeading = true
	r.leadingArea.Refresh()
	r.Refresh()
}

func (r *ActionRow) setLeadingResource(resource fyne.Resource) {
	r.leading.SetResource(resource)
	if favicon, ok := resource.(*model.Favicon); ok && len(favicon.Data) > 0 {
		r.SetLeadingObject(container.NewGridWrap(fyne.NewSquareSize(32), newIconPreview(favicon)))
		return
	}
	if r.customLeading {
		r.leadingArea.Objects[0] = container.NewGridWrap(fyne.NewSquareSize(32), r.leading)
		r.customLeading = false
		r.leadingArea.Refresh()
	}
}

// UseDefaultTextTheme keeps native label padding for wrapped or supplemental content.
func (r *ActionRow) UseDefaultTextTheme() {
	r.content.Objects[0] = container.NewThemeOverride(r.text, theme.Current())
	r.content.Refresh()
}

// SetSupplemental adds a compact secondary component below the detail text.
func (r *ActionRow) SetSupplemental(object fyne.CanvasObject) {
	r.supplemental = object
	if object == nil {
		r.text.Layout = layout.NewCustomPaddedVBoxLayout(0)
		r.text.Objects = []fyne.CanvasObject{r.title, r.detail}
	} else {
		text := container.New(layout.NewCustomPaddedVBoxLayout(0), r.title, r.detail)
		r.text.Layout = layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding))
		r.text.Objects = []fyne.CanvasObject{text, object}
	}
	r.text.Refresh()
	r.Refresh()
}

// SetDetailStyle applies a shared semantic typography role to the secondary line.
func (r *ActionRow) SetDetailStyle(style textStyle) {
	r.detail.SizeName = style.size
	r.detail.TextStyle = style.weight
	r.detail.Importance = style.importance
	r.detail.Wrapping = style.wrapping
	r.detail.Truncation = fyne.TextTruncateOff
	r.detail.Refresh()
	r.Refresh()
}

// SetContent updates the displayed data when a list reuses this row.
func (r *ActionRow) SetContent(title, detail string, resource fyne.Resource) {
	r.title.SetText(title)
	r.detail.SetText(detail)
	r.setLeadingResource(resource)
	r.Refresh()
}

// SetTapped changes the action when a list recycles this row for another item.
func (r *ActionRow) SetTapped(tapped func()) {
	r.button.OnTapped = tapped
}

// SetTrailingText shows a compact value before the row's navigation chevron.
func (r *ActionRow) SetTrailingText(text string) {
	if text == "" {
		r.count.Hide()
		r.trailing.Layout = layout.NewCenterLayout()
		r.trailing.Objects = []fyne.CanvasObject{r.trailingArrow()}
	} else {
		r.count.SetText(text)
		r.count.Show()
		r.trailing.Layout = layout.NewHBoxLayout()
		r.trailing.Objects = []fyne.CanvasObject{r.count, r.trailingArrow()}
	}
	if r.interactive || text != "" {
		r.trailing.Show()
	} else {
		r.trailing.Hide()
	}
	r.Refresh()
}

func (r *ActionRow) trailingArrow() *fyne.Container {
	return container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(theme.Size(theme.SizeNameInlineIcon)), r.next))
}

// SetInteractive controls navigation and interaction while preserving the row's styling.
func (r *ActionRow) SetInteractive(interactive bool) {
	r.interactive = interactive
	if !interactive {
		r.button.Hide()
		r.next.Hide()
		if r.count.Visible() {
			r.trailing.Show()
		} else {
			r.trailing.Hide()
		}
	} else {
		r.button.Show()
		r.next.Show()
		r.trailing.Show()
	}
}

// SetFlat removes the outline while keeping the full-row button highlight.
func (r *ActionRow) SetFlat(flat bool) {
	r.flat = flat
	if flat {
		r.title.Wrapping = fyne.TextWrapOff
		r.title.Truncation = fyne.TextTruncateEllipsis
		r.detail.Wrapping = fyne.TextWrapOff
		r.detail.Truncation = fyne.TextTruncateEllipsis
	} else {
		r.title.Wrapping = titleStyle.wrapping
		r.title.Truncation = fyne.TextTruncateOff
		r.detail.Wrapping = bodyStyle.wrapping
		r.detail.Truncation = fyne.TextTruncateOff
	}
	r.Refresh()
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
	if r.row.flat {
		r.outline.StrokeColor = color.Transparent
		r.outline.StrokeWidth = 0
		inset = 0
	}
	r.padded.Layout = layout.NewCustomPaddedLayout(inset, inset, inset, inset)
	r.WidgetRenderer.Refresh()
}
