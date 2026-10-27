// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/v2"
)

// unlockerVaultWidget is a widget that allows the user to unlock a vault
type unlockerVaultWidget struct {
	widget.BaseWidget
	app       *app
	vaultInfo storage.VaultInfo
	back      func()
	button    *widget.Button
	password  *widget.Entry
}

// unlock reads the password and opens the selected vault.
func (uw *unlockerVaultWidget) unlock() {
	if uw.password.Disabled() {
		return
	}
	password := uw.password.Text
	if password == "" {
		dialog.ShowError(errors.New("the password is required"), uw.app.win)
		return
	}

	act := NewActivity()
	uw.button.Disable()
	uw.password.Disable()
	message := "Unlocking vault..."
	if uw.vaultInfo.NeedsMigration {
		message = "Migrating vault..."
	}
	d := dialog.NewCustomWithoutButtons(message, act, uw.app.win)
	act.Start()
	d.Show()

	go func() {
		key, err := uw.app.storage.LoadVaultKey(uw.vaultInfo.ID, password)
		if err != nil {
			var invalidPasswordError *age.NoIdentityMatchError
			if errors.As(err, &invalidPasswordError) {
				err = errors.New("the password is incorrect")
			}
			fyne.Do(func() {
				act.Stop()
				d.Hide()
				uw.button.Enable()
				uw.password.Enable()
				dialog.ShowError(err, uw.app.win)
			})
			return
		}
		vault, err := uw.app.storage.LoadVault(uw.vaultInfo.ID, key)
		fyne.Do(func() {
			act.Stop()
			d.Hide()
			if err != nil {
				uw.button.Enable()
				uw.password.Enable()
				dialog.ShowError(err, uw.app.win)
				return
			}
			uw.app.setVaultView(vault, vault.Accounts[0].ID)
			uw.app.showCurrentVaultView()
			uw.app.addSSHKeysToAgent()
		})
	}()
}

// NewUnlockerVaultWidget creates the password form for a locked vault.
func NewUnlockerVaultWidget(info storage.VaultInfo, a *app) *unlockerVaultWidget {

	uw := &unlockerVaultWidget{
		app:       a,
		vaultInfo: info,
	}
	uw.ExtendBaseWidget(uw)
	uw.button = widget.NewButton("Unlock", uw.unlock)
	uw.button.Importance = widget.HighImportance
	uw.button.Disable()
	password := widget.NewPasswordEntry()
	password.Validator = func(value string) error {
		if value == "" {
			return errors.New("master password is required")
		}
		return nil
	}
	password.OnChanged = func(value string) {
		if value == "" || password.Disabled() {
			uw.button.Disable()
		} else {
			uw.button.Enable()
		}
	}
	password.OnSubmitted = func(s string) {
		if !uw.button.Disabled() {
			uw.unlock()
		}
	}
	uw.password = password
	return uw
}

// CreateRenderer builds the vault unlock form.
func (uw *unlockerVaultWidget) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(v2.NewUnlockVault(uw.vaultInfo.Name, uw.password, uw.button, uw.back, uw.app.showAddVaultView))
}
