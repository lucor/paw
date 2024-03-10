// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makeCreateVaultView() fyne.CanvasObject {
	logo := pawLogo()

	heading := headingText("Create a new vault")

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	btn := widget.NewButton("Create", func() {
		key, err := a.storage.CreateVaultKey(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		vault, err := a.storage.CreateVault(name.Text, key)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.setVaultView(vault)
		a.showCurrentVaultView()
		a.win.SetMainMenu(a.makeMainMenu())
		a.makeSysTray()
	})

	return container.NewCenter(container.NewVBox(logo, heading, name, password, btn))
}

func (a *app) makeSelectVaultView(vaults []string) fyne.CanvasObject {
	heading := headingText("Select a Vault")
	heading.Alignment = fyne.TextAlignCenter

	c := container.NewVBox(pawLogo(), heading)

	for _, v := range vaults {
		name := v
		resource := icon.LockOpenOutlinedIconThemed
		if _, ok := a.unlockedVault[name]; !ok {
			resource = icon.LockOutlinedIconThemed
		}
		btn := widget.NewButtonWithIcon(name, resource, func() {
			a.setVaultViewByName(name)
		})
		btn.Alignment = widget.ButtonAlignLeading
		c.Add(btn)
	}
	return container.NewCenter(c)
}

func (a *app) makeUnlockVaultView(vaultName string) fyne.CanvasObject {
	return NewUnlockerVaultWidget(vaultName, a)
}

func (a *app) makeCurrentVaultView() fyne.CanvasObject {
	vault := a.vault
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
						a.removeSSHKeyFromAgent(item)     // remove item from ssh agent
						a.storage.DeleteItem(vault, item) // remove item from storage
						a.storage.StoreVault(vault)       // ensure vault is up-to-date
						itemsWidget.Reload(nil, filter)
					}
				},
				a.win,
			).Show()
			return
		}

		fyneItem := NewFyneItem(item, a.config)
		a.showItemView(fyneItem)
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
		name := fmt.Sprintf("%s (%d)", t.Label(), vault.SizeByType(t))
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

	header := container.NewBorder(nil, nil, nil, a.makeVaultMenu(), list)

	button := widget.NewButtonWithIcon("Add item", theme.ContentAddIcon(), func() {
		a.showAddItemView()
	})

	// layout so we can focus the search box using shift+tab
	return container.NewBorder(search, nil, nil, nil, container.NewBorder(header, button, nil, nil, itemsWidget))
}
