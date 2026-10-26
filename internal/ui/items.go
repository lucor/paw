// SPDX-FileCopyrightText: 2021-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

type itemsWidget struct {
	widget.BaseWidget
	vault      *model.Vault
	view       *fyne.Container
	listEntry  *widget.List
	OnSelected func(*cxf.Item)
}

// newItemsWidget creates a filtered list of the vault's item summaries.
func newItemsWidget(vault *model.Vault, opts *paw.VaultFilterOptions) *itemsWidget {
	iw := &itemsWidget{vault: vault, OnSelected: func(*cxf.Item) {}}
	iw.listEntry = iw.makeList(opts)
	iw.view = container.NewStack(iw.listEntry)
	iw.ExtendBaseWidget(iw)
	return iw
}

// CreateRenderer returns the list view renderer.
func (iw *itemsWidget) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(iw.view) }

// Length returns the number of visible item summaries.
func (iw *itemsWidget) Length() int { return iw.listEntry.Length() }

// Reload rebuilds the item list with the selected filters.
func (iw *itemsWidget) Reload(_ *cxf.Item, opts *paw.VaultFilterOptions) {
	iw.listEntry = iw.makeList(opts)
	iw.view.Objects[0] = iw.listEntry
	iw.view.Refresh()
}

// makeList creates the Fyne list from matching vault summaries.
func (iw *itemsWidget) makeList(opts *paw.VaultFilterOptions) *widget.List {
	items, err := iw.vault.FilterItems(opts.Name, opts.ItemType)
	if err != nil {
		items = []cxf.Item{}
	}
	sort.Slice(items, func(i, j int) bool { return strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title) })
	list := widget.NewList(func() int { return len(items) }, func() fyne.CanvasObject {
		return newItemListContainer(widget.NewIcon(icon.LockOutlinedIconThemed), &canvas.Text{TextSize: theme.TextSize(), TextStyle: fyne.TextStyle{Bold: true}}, &canvas.Text{TextSize: theme.CaptionTextSize()})
	}, func(id int, obj fyne.CanvasObject) {
		item := &items[id]
		iconWidget := obj.(*fyne.Container).Objects[0].(*widget.Icon)
		iconWidget.SetResource(summaryIcon(item))
		title := obj.(*fyne.Container).Objects[1].(*canvas.Text)
		title.Text = item.Title
		title.Refresh()
		subtitle := obj.(*fyne.Container).Objects[2].(*canvas.Text)
		if item.Subtitle != nil {
			subtitle.Text = *item.Subtitle
		} else {
			subtitle.Text = ""
		}
		subtitle.Refresh()
	})
	list.OnSelected = func(id widget.ListItemID) { iw.OnSelected(&items[id]) }
	return list
}

// summaryIcon returns the icon for a summary's Paw item category.
func summaryIcon(item *cxf.Item) fyne.Resource {
	typeOfItem, err := model.ItemTypeOf(item)
	if err != nil {
		return icon.PawIcon
	}
	if typeOfItem == paw.LoginItemType {
		for _, raw := range item.Extensions {
			var extension struct {
				Name    string       `json:"name"`
				Favicon *paw.Favicon `json:"favicon"`
			}
			if json.Unmarshal(raw, &extension) == nil && extension.Name == "paw.pm/item" && extension.Favicon != nil {
				return extension.Favicon
			}
		}
	}
	switch typeOfItem {
	case paw.NoteItemType:
		return icon.NoteOutlinedIconThemed
	case paw.LoginItemType:
		return icon.WorldWWWOutlinedIconThemed
	case paw.PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case paw.SSHKeyItemType:
		return icon.KeyOutlinedIconThemed
	}
	return icon.PawIcon
}

var iconBoxSize = fyne.NewSize(36, 36)
var iconSize = fyne.NewSize(24, 24)

type itemListLayout struct {
	icon            *widget.Icon
	title, subtitle *canvas.Text
}

// MinSize returns enough room for the icon, title, and subtitle.
func (l *itemListLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	height := fyne.Max(l.title.MinSize().Height, l.subtitle.MinSize().Height)
	width := fyne.Max(l.title.MinSize().Width, l.subtitle.MinSize().Width)
	return fyne.NewSize(iconBoxSize.Width+width, fyne.Max(iconBoxSize.Height, height*2))
}

// Layout positions the icon and text in one item row.
func (l *itemListLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	l.icon.Resize(iconSize)
	l.icon.Move(fyne.NewPos((iconBoxSize.Width-l.icon.MinSize().Width)/2, (size.Height-l.icon.MinSize().Height)/2))
	left := iconBoxSize.Width + theme.InnerPadding()
	if l.subtitle.Text == "" {
		l.title.Move(fyne.NewPos(left, (size.Height-l.title.MinSize().Height)/2))
		return
	}
	l.title.Move(fyne.NewPos(left, theme.SeparatorThicknessSize()))
	l.subtitle.Move(fyne.NewPos(left, size.Height/2))
}

// newItemListContainer creates the icon and text container for one summary.
func newItemListContainer(i *widget.Icon, t, s *canvas.Text) *fyne.Container {
	return container.New(&itemListLayout{icon: i, title: t, subtitle: s}, i, t, s)
}
