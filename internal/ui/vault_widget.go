// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"fmt"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/storage"
)

// unlockerVaultWidget is a widget that allows the user to unlock a vault
type unlockerVaultWidget struct {
	widget.BaseWidget
	app       *app
	vaultInfo storage.VaultInfo
	icon      *canvas.Image
	button    *widget.Button
	password  *widget.Entry
}

// unlock reads the password and opens the selected vault.
func (uw *unlockerVaultWidget) unlock() {
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
		icon:      pawLogo(),
		vaultInfo: info,
	}
	uw.ExtendBaseWidget(uw)
	uw.button = widget.NewButtonWithIcon("Unlock", icon.LockOpenOutlinedIconThemed, uw.unlock)
	password := widget.NewPasswordEntry()
	password.SetPlaceHolder("Password")
	password.OnSubmitted = func(s string) {
		uw.unlock()
	}
	uw.password = password
	return uw
}

// CreateRenderer builds the vault unlock form.
func (uw *unlockerVaultWidget) CreateRenderer() fyne.WidgetRenderer {
	msg := fmt.Sprintf("Vault %q is locked", uw.vaultInfo.Name)
	heading := headingText(msg)

	c := container.NewCenter(container.NewVBox(uw.icon, heading, uw.password, uw.button))
	return widget.NewSimpleRenderer(c)
}
