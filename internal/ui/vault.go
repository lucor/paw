// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

// makeCreateVaultView displays the form for creating a vault.
func (a *app) makeCreateVaultView() fyne.CanvasObject {
	logo := pawLogo()

	heading := headingText("Create a new vault")

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	btn := widget.NewButton("Create", func() {
		vault, err := a.storage.CreateVault(name.Text, password.Text)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.setVaultView(vault, vault.Accounts[0].ID)
		a.showCurrentVaultView()
		a.win.SetMainMenu(a.makeMainMenu())
		a.makeSysTray()
	})

	return container.NewCenter(container.NewVBox(logo, heading, name, password, btn))
}

// makeSelectVaultView displays the available vaults.
func (a *app) makeSelectVaultView(vaults []storage.VaultInfo) fyne.CanvasObject {
	heading := headingText("Select a Vault")
	heading.Alignment = fyne.TextAlignCenter

	c := container.NewVBox(pawLogo(), heading)

	for _, vault := range vaults {
		info := vault
		resource := icon.LockOpenOutlinedIconThemed
		if _, ok := a.unlockedVault[info.ID]; !ok {
			resource = icon.LockOutlinedIconThemed
		}
		btn := widget.NewButtonWithIcon(info.Name, resource, func() {
			a.setVaultViewByInfo(info)
		})
		btn.Alignment = widget.ButtonAlignLeading
		c.Add(btn)
	}
	return container.NewCenter(c)
}

// makeUnlockVaultView displays the password form for a locked vault.
func (a *app) makeUnlockVaultView(info storage.VaultInfo) fyne.CanvasObject {
	return NewUnlockerVaultWidget(info, a)
}

// makeCurrentVaultView displays the current vault and its item summaries.
func (a *app) makeCurrentVaultView() fyne.CanvasObject {
	vault := a.vault
	filter, ok := a.filter[a.vaultID]
	if !ok {
		filter = &paw.VaultFilterOptions{}
		a.filter[a.vaultID] = filter
	}

	itemsWidget := newItemsWidget(vault, filter)
	itemsWidget.OnSelected = func(summary *cxf.Item) {
		item, err := a.storage.LoadItem(vault, summary.ID)
		if err != nil {
			msg := fmt.Sprintf("error loading %q.\nDo you want delete from the vault?", summary.Title)
			fyne.LogError("error loading item from vault", err)
			dialog.NewConfirm(
				"Error",
				msg,
				func(delete bool) {
					if delete {
						if err := a.storage.DeleteItem(vault, summary.ID); err != nil {
							dialog.ShowError(err, a.win)
							return
						}
						now := time.Now().UTC()
						a.state.Modified = now
						_ = a.storage.StoreAppState(a.state)
						itemsWidget.Reload(nil, filter)
					}
				},
				a.win,
			).Show()
			return
		}

		fyneItemWidget := NewFyneItemWidget(item, a.state.Preferences)
		a.showItemView(fyneItemWidget)
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
	options := []string{fmt.Sprintf("All items (%d)", len(vault.Accounts[0].Items))}
	for _, t := range a.makeEmptyItems() {
		count := 0
		for _, summary := range vault.Accounts[0].Items {
			itemType, _ := model.ItemTypeOf(&summary)
			if itemType == t {
				count++
			}
		}
		name := fmt.Sprintf("%s (%d)", t.Label(), count)
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

// makeEmptyItems returns the item categories supported by the editor.
func (a *app) makeEmptyItems() []paw.ItemType {
	return []paw.ItemType{paw.NoteItemType, paw.PasswordItemType, paw.LoginItemType, paw.SSHKeyItemType}
}
