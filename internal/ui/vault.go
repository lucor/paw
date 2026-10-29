// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// showCreateVaultView opens the vault creation screen.
func (a *app) showCreateVaultView() {
	a.closeItemDetail()
	a.win.SetContent(NewCreateVault(a.showCurrentVaultView, a.createVault))
}

// showAddVaultView opens the available ways to add a vault.
func (a *app) showAddVaultView() {
	a.closeItemDetail()
	ShowAddVault(a.win, a.showCurrentVaultView, a.createVault, a.showRestoreVaultView)
}

// showRestoreVaultView restores a Paw backup as a new unlocked vault.
func (a *app) showRestoreVaultView() {
	a.closeItemDetail()
	ShowRestoreBackup(a.win, a.storage, func(vault *model.Vault) {
		a.setVaultView(vault, vault.Accounts[0].ID)
		a.showCurrentVaultView()
		a.win.SetMainMenu(a.makeMainMenu())
		a.makeSysTray()
	})
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
	a.showEditItemView(item, true)
}

// makeSelectVaultView displays the available vaults.
func (a *app) makeSelectVaultView(vaults []storage.VaultInfo) fyne.CanvasObject {
	choices := make([]VaultChoice, 0, len(vaults))
	for _, vault := range vaults {
		info := vault
		_, unlocked := a.unlockedVault[info.ID]
		choices = append(choices, VaultChoice{Name: info.Name, Unlocked: unlocked, Open: func() {
			a.setVaultViewByInfo(info, func() {
				a.main.Content = a.makeSelectVaultView(vaults)
				a.main.Refresh()
				a.showCurrentVaultView()
			})
		}})
	}
	return NewSelectVault(choices, a.showAddVaultView)
}

// makeUnlockVaultView displays the password form for a locked vault.
func (a *app) makeUnlockVaultView(info storage.VaultInfo, back func()) fyne.CanvasObject {
	return NewUnlockVault(info, a.storage, func(vault *model.Vault) {
		a.setVaultView(vault, info.ID)
		a.showCurrentVaultView()
		a.addSSHKeysToAgent()
	}, back, a.showAddVaultView)
}

// makeCurrentVaultView displays the current vault and its item summaries.
func (a *app) makeCurrentVaultView() fyne.CanvasObject {
	a.closeCodesView()
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

		a.showItemView(item)
	}
	categoryIDs := map[paw.ItemType]string{0: "all", paw.LoginItemType: "login", paw.PasswordItemType: "password", paw.NoteItemType: "note", paw.SSHKeyItemType: "ssh"}
	var rows []EntryRow
	var codeEntries []codesEntry
	for _, account := range vault.Accounts {
		for _, item := range account.Items {
			summary := item
			kind, _ := model.ItemTypeOf(&summary)
			subtitle := ""
			if summary.Subtitle != nil {
				subtitle = *summary.Subtitle
			}
			rows = append(rows, EntryRow{Title: summary.Title, Subtitle: subtitle, Category: categoryIDs[kind], Icon: summaryIcon(&summary), Open: func() { openItem(&summary) }})
			if kind != paw.LoginItemType {
				continue
			}
			loaded, err := a.storage.LoadItem(vault, summary.ID)
			if err != nil {
				log.Printf("unable to load login TOTP for %q: %v", summary.Title, err)
				continue
			}
			credentials, err := decodeEntryDetailCredentials(loaded.Credentials)
			if err != nil || credentials.totp == nil {
				continue
			}
			username := credentials.totp.Username
			if username == "" {
				username = subtitle
			}
			codeEntries = append(codeEntries, codesEntry{Title: summary.Title, Username: username, ItemID: summary.ID, Icon: summaryIcon(&summary), Credential: *credentials.totp})
		}
	}
	codesContext, codesCancel := context.WithCancel(context.Background())
	a.codesCancel = codesCancel
	a.codesView = newCodesView(codesContext, codeEntries, a)
	vaults, err := a.storage.Vaults()
	if err != nil {
		fyne.LogError("Unable to list vaults", err)
	}
	choices := make([]VaultChoice, 0, len(vaults))
	currentID := a.vaultID
	for _, info := range vaults {
		_, unlocked := a.unlockedVault[info.ID]
		choices = append(choices, VaultChoice{Name: info.Name, Unlocked: unlocked, Active: info.ID == currentID, Open: func() {
			a.setVaultViewByInfo(info, func() { a.setVaultViewByInfo(storage.VaultInfo{ID: currentID}, nil) })
		}})
	}
	name, _ := vault.Name()
	categories := []EntryCategory{{ID: "all", Label: "All items", Icon: icons.Vault}, {ID: "login", Label: "Logins", Icon: icons.Login}, {ID: "password", Label: "Passwords", Icon: icons.Password}, {ID: "note", Label: "Notes", Icon: icons.Note}, {ID: "ssh", Label: "SSH keys", Icon: icons.SSH}}
	return NewEntryList(EntryListOptions{VaultName: name, Vaults: choices, VaultActions: a.makeVaultActions(), Entries: rows, Categories: categories, Search: filter.Name, Category: categoryIDs[filter.ItemType], Codes: a.codesView.View, AddEntry: func(kind EntryKind) {
		switch kind {
		case EntryNote:
			a.addItem(paw.NoteItemType)
		case EntryPassword:
			a.addItem(paw.PasswordItemType)
		case EntryLogin:
			a.addItem(paw.LoginItemType)
		case EntrySSH:
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
