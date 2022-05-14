package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makeVaultView(vault *paw.Vault) fyne.CanvasObject {

	filter, ok := a.filter[vault.Name]
	if !ok {
		filter = &paw.VaultFilterOptions{}
		a.filter[vault.Name] = filter
	}

	itemsWidget := newItemsWidget(vault, filter)
	itemsWidget.OnSelected = func(meta *paw.Metadata) {
		item, err := a.storage.LoadItem(vault, meta)
		if err != nil {
			msg := fmt.Sprintf("error loading %q.\nDo you want delete from the vault?", meta.Name)
			fyne.LogError("error loading item from vault", err)
			dialog.NewConfirm(
				"Error",
				msg,
				func(delete bool) {
					if delete {
						item, _ = paw.NewItem(meta.Name, meta.Type)
						vault.DeleteItem(item)            // remove item from vault
						a.storage.DeleteItem(vault, item) // remove item from storage
						a.storage.StoreVault(vault)       // ensure vault is up-to-date
						itemsWidget.Reload(nil, filter)
					}
				},
				a.win,
			).Show()
			return
		}

		fyneItem := NewFyneItem(item)
		a.setContent(a.makeShowItemView(fyneItem))
		itemsWidget.listEntry.UnselectAll()
	}

	// search entries
	search := widget.NewEntry()
	search.SetPlaceHolder("Search")
	search.SetText(filter.Name)
	search.OnChanged = func(s string) {
		filter.Name = s
		itemsWidget.Reload(nil, filter)
	}

	// filter entries
	itemTypeMap := map[string]paw.ItemType{}
	options := []string{fmt.Sprintf("All items (%d)", vault.Size())}
	for _, item := range a.makeEmptyItems() {
		i := item
		t := i.GetMetadata().Type
		name := fmt.Sprintf("%s (%d)", strings.Title(t.String()), vault.SizeByType(t))
		options = append(options, name)
		itemTypeMap[name] = t
	}

	list := widget.NewSelect(options, func(s string) {
		var v paw.ItemType
		if s == options[0] {
			v = paw.ItemType(0) // No item type will be selected
		} else {
			v = itemTypeMap[s]
		}

		filter.ItemType = v
		itemsWidget.Reload(nil, filter)
	})

	list.SetSelectedIndex(0)

	return container.NewBorder(container.NewVBox(a.makeVaultMenu(), search, list), nil, nil, nil, itemsWidget)
}
