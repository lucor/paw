package ui

import (
	"errors"
	"fmt"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makeCreateVaultView() fyne.CanvasObject {
	logo := pawLogo()

	name := widget.NewEntry()
	name.SetPlaceHolder("Name")

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	btn := widget.NewButton("Create Vault", func() {
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

	return container.NewCenter(container.NewVBox(logo, name, password, btn))
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
	logo := pawLogo()

	msg := fmt.Sprintf("Vault %q is locked", vaultName)
	heading := headingText(msg)

	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")

	unlockBtn := widget.NewButtonWithIcon("Unlock", icon.LockOpenOutlinedIconThemed, func() {
		key, err := a.storage.LoadVaultKey(vaultName, password.Text)
		if err != nil {
			var invalidPasswordError *age.NoIdentityMatchError
			if errors.As(err, &invalidPasswordError) {
				err = errors.New("the password is incorrect")
			}
			dialog.ShowError(err, a.win)
			return
		}
		vault, err := a.storage.LoadVault(vaultName, key)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.setVaultView(vault)
		a.addSSHKeysToAgent(vault)
		a.showCurrentVaultView()
	})

	return container.NewCenter(container.NewVBox(logo, heading, password, unlockBtn))
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
	ca := cases.Title(language.Und)
	for _, item := range a.makeEmptyItems() {
		i := item
		t := i.GetMetadata().Type
		name := fmt.Sprintf("%s (%d)", ca.String(t.String()), vault.SizeByType(t))
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

	header := container.NewVBox(
		container.NewBorder(nil, nil, nil, a.makeVaultMenu(), search),
		list,
	)

	button := widget.NewButtonWithIcon("Add item", theme.ContentAddIcon(), func() {
		a.showAddItemView()
	})

	return container.NewBorder(header, button, nil, nil, itemsWidget)
}
