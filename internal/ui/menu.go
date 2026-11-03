// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"log"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"go.lucor.dev/paw/internal/model"
)

const (
	menuFileLabel        = "File"
	menuVaultLabel       = "Vault"
	menuNewVaultLabel    = "New Vault"
	menuSettingsLabel    = "Settings"
	menuCloseWindowLabel = "Close Window"
	menuQuitLabel        = "Quit"
	menuHelpLabel        = "Help"
	menuAboutLabel       = "About" // Fyne maps this exact label to the native About Paw item.
)

func (a *app) makeMainMenu() *fyne.MainMenu {
	// a Quit item will is appended automatically by Fyne to the first menu item
	vaultItem := fyne.NewMenuItem(menuVaultLabel, nil)
	vaultItem.ChildMenu = fyne.NewMenu("", a.makeVaultMenuItems()...)

	fileMenu := fyne.NewMenu(menuFileLabel,
		fyne.NewMenuItem(menuNewVaultLabel, func() {
			a.showCreateVaultView()
		}),
		vaultItem,
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(menuSettingsLabel, func() {
			a.showSettingsView()
		}),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(menuCloseWindowLabel, func() {
			a.win.Hide()
		}),
		fyne.NewMenuItem(menuQuitLabel, func() {
			a.win.SetCloseIntercept(nil)
			a.win.Close()
		}),
	)

	helpMenu := fyne.NewMenu(menuHelpLabel,
		fyne.NewMenuItem(menuAboutLabel, func() { a.showAboutPawView(a.win.Content()) }),
	)

	return fyne.NewMainMenu(
		fileMenu,

		helpMenu,
	)
}
func (a *app) makeVaultActions() []*fyne.MenuItem {
	return NewVaultActions(func() { a.showVaultSettingsView() }, a.showAuditPasswordView, a.exportToFile, func() {
		name, _ := a.vault.Name()
		a.main.Content = a.makeUnlockVaultView(model.VaultInfo{ID: a.vaultID, Name: name}, nil, nil)
		a.lockVault()
		a.main.Refresh()
	})
}

func (a *app) makeVaultMenuItems() []*fyne.MenuItem {
	vaults, err := a.storage.Vaults()
	if err != nil {
		log.Fatal(err)
	}

	mi := make([]*fyne.MenuItem, len(vaults))
	var chooseAnother func()
	if len(vaults) >= 2 {
		chooseAnother = func() { a.showVaultSelector(vaults) }
	}
	for i, vault := range vaults {
		i := i
		info := vault
		mi[i] = fyne.NewMenuItem(info.Name, func() {})
		_, isDesktop := fyne.CurrentApp().(desktop.App)
		if isDesktop && i < 9 {
			shortcut := &desktop.CustomShortcut{KeyName: fyne.KeyName(fmt.Sprint(i + 1)), Modifier: fyne.KeyModifierControl}
			a.win.Canvas().AddShortcut(shortcut, func(shortcut fyne.Shortcut) {
				a.setVaultViewByInfo(info, nil, chooseAnother)
			})
			mi[i].Shortcut = shortcut
		}
		mi[i].Action = func() {
			defer a.win.Show()
			a.setVaultViewByInfo(info, nil, chooseAnother)
		}
	}
	return mi
}
