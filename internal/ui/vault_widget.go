// Copyright 2024 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
	"lucor.dev/paw/internal/icon"
)

// unlockerVaultWidget is a widget that allows the user to unlock a vault
type unlockerVaultWidget struct {
	widget.BaseWidget
	app       *app
	vaultName string
	icon      *canvas.Image
	button    *widget.Button
	password  *widget.Entry
}

// unlock unlocks the vault
func (uw *unlockerVaultWidget) unlock() {
	password := uw.password.Text
	if password == "" {
		dialog.ShowError(errors.New("the password is required"), uw.app.win)
		return
	}

	act := NewActivity()
	d := dialog.NewCustomWithoutButtons("unlocking vault...", act, uw.app.win)
	act.Start()
	d.Show()

	stopAct := func() {
		act.Stop()
		d.Hide()
	}

	key, err := uw.app.storage.LoadVaultKey(uw.vaultName, password)
	if err != nil {
		var invalidPasswordError *age.NoIdentityMatchError
		if errors.As(err, &invalidPasswordError) {
			err = errors.New("the password is incorrect")
		}
		stopAct()
		dialog.ShowError(err, uw.app.win)
		return
	}
	vault, err := uw.app.storage.LoadVault(uw.vaultName, key)
	if err != nil {
		stopAct()
		dialog.ShowError(err, uw.app.win)
		return
	}
	uw.app.setVaultView(vault)
	uw.app.addSSHKeysToAgent()
	stopAct()
	uw.app.showCurrentVaultView()
}

// NewUnlockerVaultWidget creates a new unlockerVaultWidget
func NewUnlockerVaultWidget(vaultName string, a *app) *unlockerVaultWidget {

	uw := &unlockerVaultWidget{
		app:       a,
		icon:      pawLogo(),
		vaultName: vaultName,
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

// CreateRenderer creates a new renderer for the unlockerVaultWidget
func (uw *unlockerVaultWidget) CreateRenderer() fyne.WidgetRenderer {
	msg := fmt.Sprintf("Vault %q is locked", uw.vaultName)
	heading := headingText(msg)

	c := container.NewCenter(container.NewVBox(uw.icon, heading, uw.password, uw.button))
	return widget.NewSimpleRenderer(c)
}
