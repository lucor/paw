// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/v2"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
)

// showCreateVaultView opens the vault creation screen.
func (a *app) showCreateVaultView() {
	a.win.SetContent(v2.NewCreateVault(a.showCurrentVaultView, a.createVault))
}

// showAddVaultView opens the available ways to add a vault.
func (a *app) showAddVaultView() {
	v2.ShowAddVault(a.win, a.showCurrentVaultView, a.createVault)
}

// createVault stores a vault and makes it the active vault.
func (a *app) createVault(name, password string) error {
	vault, err := a.storage.CreateVault(name, password)
	if err != nil {
		return err
	}
	a.setVaultView(vault, vault.Accounts[0].ID)
	a.showCurrentVaultView()
	a.win.SetMainMenu(a.makeMainMenu())
	a.makeSysTray()
	return nil
}

// addItem creates an item of the selected type and opens its editor.
func (a *app) addItem(itemType paw.ItemType) {
	item, err := newItemTemplate(itemType)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.showEditItemView(newModelItemWidget(item, a.state.Preferences, true))
}

// makeSelectVaultView displays the available vaults.
func (a *app) makeSelectVaultView(vaults []storage.VaultInfo) fyne.CanvasObject {
	choices := make([]v2.VaultChoice, 0, len(vaults))
	for _, vault := range vaults {
		info := vault
		_, unlocked := a.unlockedVault[info.ID]
		choices = append(choices, v2.VaultChoice{Name: info.Name, Unlocked: unlocked, Open: func() {
			a.setVaultViewByInfo(info, func() {
				a.main.Content = a.makeSelectVaultView(vaults)
				a.main.Refresh()
				a.showCurrentVaultView()
			})
		}})
	}
	return v2.NewSelectVault(choices, a.showAddVaultView)
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

	openItem := func(summary *cxf.Item) {
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
						a.main.Content = a.makeCurrentVaultView()
						a.main.Refresh()
					}
				},
				a.win,
			).Show()
			return
		}

		fyneItemWidget := NewFyneItemWidget(item, a.state.Preferences)
		a.showItemView(fyneItemWidget)
	}
	categoryIDs := map[paw.ItemType]string{0: "all", paw.LoginItemType: "login", paw.PasswordItemType: "password", paw.NoteItemType: "note", paw.SSHKeyItemType: "ssh"}
	var rows []v2.EntryRow
	for _, account := range vault.Accounts {
		for _, item := range account.Items {
			summary := item
			kind, _ := model.ItemTypeOf(&summary)
			subtitle := ""
			if summary.Subtitle != nil {
				subtitle = *summary.Subtitle
			}
			rows = append(rows, v2.EntryRow{Title: summary.Title, Subtitle: subtitle, Category: categoryIDs[kind], Icon: summaryIcon(&summary), Open: func() { openItem(&summary) }})
		}
	}
	vaults, err := a.storage.Vaults()
	if err != nil {
		fyne.LogError("Unable to list vaults", err)
	}
	choices := make([]v2.VaultChoice, 0, len(vaults))
	currentID := a.vaultID
	for _, info := range vaults {
		_, unlocked := a.unlockedVault[info.ID]
		choices = append(choices, v2.VaultChoice{Name: info.Name, Unlocked: unlocked, Active: info.ID == currentID, Open: func() {
			a.setVaultViewByInfo(info, func() { a.setVaultViewByInfo(storage.VaultInfo{ID: currentID}, nil) })
		}})
	}
	name, _ := vault.Name()
	categories := []v2.EntryCategory{{ID: "all", Label: "All items", Icon: icons.Vault}, {ID: "login", Label: "Logins", Icon: icons.Login}, {ID: "password", Label: "Passwords", Icon: icons.Password}, {ID: "note", Label: "Notes", Icon: icons.Note}, {ID: "ssh", Label: "SSH keys", Icon: icons.SSH}}
	return v2.NewEntryList(v2.EntryListOptions{VaultName: name, Vaults: choices, VaultActions: a.makeVaultActions(), Entries: rows, Categories: categories, Search: filter.Name, Category: categoryIDs[filter.ItemType], AddEntry: func(kind v2.EntryKind) {
		switch kind {
		case v2.EntryNote:
			a.addItem(paw.NoteItemType)
		case v2.EntryPassword:
			a.addItem(paw.PasswordItemType)
		case v2.EntryLogin:
			a.addItem(paw.LoginItemType)
		case v2.EntrySSH:
			a.addItem(paw.SSHKeyItemType)
		}
	}, AddVault: a.showAddVaultView, OnFilterChanged: func(search, category string) {
		filter.Name = search
		for kind, id := range categoryIDs {
			if id == category {
				filter.ItemType = kind
				break
			}
		}
	}})
}
