package ui

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
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
	iw.view = container.NewMax(iw.listEntry)
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
			return container.NewHBox(widget.NewIcon(icon.LockOutlinedIconThemed), widget.NewLabel("Identity label"))
		},
		func(id int, obj fyne.CanvasObject) {
			metadata := itemMetadata[id]
			obj.(*fyne.Container).Objects[0].(*widget.Icon).SetResource(metadata.Icon())
			obj.(*fyne.Container).Objects[1].(*widget.Label).SetText(metadata.String())
		})

	if selectedItem != nil {
		for i, metadata := range itemMetadata {
			if selectedItem.ID() == metadata.ID() {
				iw.selectedIndex = i
				break
			}
		}
		list.Select(iw.selectedIndex)
	} else {
		iw.selectedIndex = -1
		list.UnselectAll()
	}

	list.OnSelected = func(id widget.ListItemID) {
		metadata := itemMetadata[id]
		iw.selectedIndex = id
		iw.OnSelected(metadata)
	}

	return list
}
