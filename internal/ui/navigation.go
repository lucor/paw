// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// NavigationItem supplies the normal and selected icons for a destination.
type NavigationItem struct {
	Icon, SelectedIcon fyne.Resource
}

// Navigation presents evenly spaced icon destinations using native buttons.
type Navigation struct {
	widget.BaseWidget
	Selected   int
	OnSelected func(int)
	items      []NavigationItem
	buttons    []*navigationButton
}

// NewNavigation creates the bottom navigation with Vault selected initially.
func NewNavigation(items ...NavigationItem) *Navigation {
	n := &Navigation{items: append([]NavigationItem(nil), items...)}
	n.ExtendBaseWidget(n)
	return n
}

// Select changes the visible destination without rebuilding its content.
func (n *Navigation) Select(index int) {
	if index < 0 || index >= len(n.items) || n.Selected == index {
		return
	}
	n.Selected = index
	for i, button := range n.buttons {
		button.selected = i == index
		button.Refresh()
	}
	if n.OnSelected != nil {
		n.OnSelected(index)
	}
}

// CreateRenderer distributes the destinations evenly across the available width.
func (n *Navigation) CreateRenderer() fyne.WidgetRenderer {
	n.buttons = make([]*navigationButton, len(n.items))
	objects := make([]fyne.CanvasObject, len(n.items))
	for i, item := range n.items {
		button := &navigationButton{item: item, selected: i == n.Selected}
		button.ExtendBaseWidget(button)
		button.button = widget.NewButton("", func() { n.Select(i) })
		button.button.Importance = widget.LowImportance
		n.buttons[i], objects[i] = button, button
	}
	columns := max(1, len(n.items))
	return widget.NewSimpleRenderer(container.NewGridWithColumns(columns, objects...))
}

type navigationButton struct {
	widget.BaseWidget
	item     NavigationItem
	selected bool
	button   *widget.Button
}

// CreateRenderer keeps a 24-pixel icon above the button's native interaction layer.
func (b *navigationButton) CreateRenderer() fyne.WidgetRenderer {
	icon := widget.NewIcon(b.item.Icon)
	indicator := canvas.NewRectangle(color.Transparent)
	indicator.SetMinSize(fyne.NewSize(28, 2))
	content := container.NewVBox(container.NewCenter(indicator), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(48), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(pawSize(pawSizeNameDetailIcon)), icon)))))
	r := &navigationButtonRenderer{WidgetRenderer: widget.NewSimpleRenderer(container.NewStack(b.button, content)), button: b, icon: icon, indicator: indicator}
	r.Refresh()
	return r
}

type navigationButtonRenderer struct {
	fyne.WidgetRenderer
	button    *navigationButton
	icon      *widget.Icon
	indicator *canvas.Rectangle
}

// Refresh follows theme changes and applies the selected destination's appearance.
func (r *navigationButtonRenderer) Refresh() {
	resource := r.button.item.Icon
	r.indicator.FillColor = color.Transparent
	if r.button.selected {
		r.indicator.FillColor = r.button.Theme().Color(theme.ColorNamePrimary, fyne.CurrentApp().Settings().ThemeVariant())
		if r.button.item.SelectedIcon != nil {
			resource = r.button.item.SelectedIcon
		}
	}
	r.icon.SetResource(resource)
	r.WidgetRenderer.Refresh()
}
