// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package ui

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// itemsWidget is a custom widget to handle the vault's item list
type itemsWidget struct {
	widget.BaseWidget

	vault *paw.Vault

	selectedIndex int

	// view is the container holds all the object rendered by this widget
	view *fyne.Container

	// list represents the item list
	listEntry *widget.List

	// OnSelected defines the callback to execute on the item list selection
	OnSelected func(*paw.Metadata)
}

// newItemsWidget returns a new items widget
func newItemsWidget(vault *paw.Vault, opts *paw.VaultFilterOptions) *itemsWidget {
	iw := &itemsWidget{
		vault:         vault,
		selectedIndex: -1,
	}
	iw.listEntry = iw.makeList(nil, opts)
	iw.view = container.NewStack(iw.listEntry)
	iw.OnSelected = func(i *paw.Metadata) {}
	iw.ExtendBaseWidget(iw)
	return iw
}

// CreateRenderer is a private method to Fyne which links this widget to its renderer
func (iw *itemsWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(iw.view)
}

// Length returns the number of items in the list
func (iw *itemsWidget) Length() int {
	return iw.listEntry.Length()
}

// Reload reloads the widget according the specified options
func (iw *itemsWidget) Reload(selectedItem paw.Item, opts *paw.VaultFilterOptions) {
	iw.listEntry = iw.makeList(selectedItem, opts)
	iw.view.Objects[0] = iw.listEntry
	iw.view.Refresh()
}

// makeList makes the Fyne list widget
func (iw *itemsWidget) makeList(selectedItem paw.Item, opts *paw.VaultFilterOptions) *widget.List {
	itemMetadata := iw.vault.FilterItemMetadata(opts)
	sort.Slice(itemMetadata, func(i, j int) bool {
		return strings.ToLower(itemMetadata[i].Name) < strings.ToLower(itemMetadata[j].Name)
	})
	list := widget.NewList(
		func() int {
			return len(itemMetadata)
		},
		func() fyne.CanvasObject {
			icon := widget.NewIcon(icon.LockOutlinedIconThemed)
			return newItemListContainer(icon, &canvas.Text{
				Text:      "Item name",
				TextSize:  theme.TextSize(),
				TextStyle: fyne.TextStyle{Bold: true},
			}, &canvas.Text{
				Text:     "Item subtitle",
				TextSize: theme.CaptionTextSize(),
			})
		},
		func(id int, obj fyne.CanvasObject) {
			metadata := &Metadata{Metadata: itemMetadata[id]}
			obj.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(metadata.Icon())
			title := obj.(*fyne.Container).Objects[1].(*canvas.Text)
			title.Text = metadata.String()
			title.Refresh()
			subtitle := obj.(*fyne.Container).Objects[2].(*canvas.Text)
			subtitle.Text = metadata.Subtitle
			subtitle.Refresh()
		})

	list.OnSelected = func(id widget.ListItemID) {
		metadata := itemMetadata[id]
		iw.OnSelected(metadata)
	}

	return list
}

var (
	iconBoxSize = fyne.NewSize(36, 36)
	iconSize    = fyne.NewSize(24, 24)
)

// itemListLayout is a custom layout for the item list
type itemListLayout struct {
	icon     *widget.Icon
	title    *canvas.Text
	subtitle *canvas.Text
}

// MinSize returns the minimum size required by the custom layout
func (l *itemListLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	boxSizeMaxHeight := fyne.Max(l.title.MinSize().Height, l.subtitle.MinSize().Height)
	boxSizeMaxWidth := fyne.Max(l.title.MinSize().Width, l.subtitle.MinSize().Width)
	return fyne.NewSize(iconBoxSize.Width+boxSizeMaxWidth, fyne.Max(iconBoxSize.Height, boxSizeMaxHeight*2))
}

// Layout arranges the objects in the custom layout
func (l *itemListLayout) Layout(objects []fyne.CanvasObject, containerSize fyne.Size) {
	l.icon.Resize(iconSize)
	l.icon.Move(fyne.NewPos((iconBoxSize.Width-l.icon.MinSize().Width)/2, (containerSize.Height-l.icon.MinSize().Height)/2))
	boxLeft := iconBoxSize.Width + theme.InnerPadding()
	if l.subtitle.Text == "" {
		l.title.Move(fyne.NewPos(boxLeft, (containerSize.Height-l.title.MinSize().Height)/2))
		return
	}
	l.title.Move(fyne.NewPos(boxLeft, theme.SeparatorThicknessSize()))
	l.subtitle.Move(fyne.NewPos(boxLeft, containerSize.Height/2))
}

// newItemListContainer returns a new container for the item list
func newItemListContainer(icon *widget.Icon, title, subtitle *canvas.Text) *fyne.Container {
	layout := &itemListLayout{
		icon:     icon,
		title:    title,
		subtitle: subtitle,
	}
	return container.New(layout, icon, title, subtitle)
}
