// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// UnlockVault displays and handles unlocking one stored vault.
type UnlockVault struct {
	widget.BaseWidget
	info       storage.VaultInfo
	store      storage.VaultStorage
	onUnlocked func(*model.Vault)
	password   *widget.Entry
	button     *widget.Button
	activity   *widget.Activity
	message    *widget.Label
	content    *fyne.Container
}

// NewUnlockVault creates a vault unlock view with navigation to Add vault.
func NewUnlockVault(info storage.VaultInfo, store storage.VaultStorage, onUnlocked func(*model.Vault), back, addVault func()) *UnlockVault {
	u := &UnlockVault{info: info, store: store, onUnlocked: onUnlocked}
	u.password = widget.NewPasswordEntry()
	u.password.Validator = func(value string) error {
		if value == "" {
			return errors.New("master password is required")
		}
		return nil
	}
	u.button = widget.NewButton("Unlock", u.unlock)
	u.button.Importance = widget.HighImportance
	u.button.Disable()
	u.activity = widget.NewActivity()
	u.activity.Hide()
	u.password.OnChanged = func(value string) {
		if value == "" || u.password.Disabled() {
			u.button.Disable()
		} else {
			u.button.Enable()
		}
	}
	u.password.OnSubmitted = func(string) {
		if !u.button.Disabled() {
			u.unlock()
		}
	}
	u.message = newText("", bodyStyle)
	u.message.Hide()
	u.content = unlockVaultContent(info, u.password, u.button, u.activity, u.message, back, addVault)
	u.ExtendBaseWidget(u)
	return u
}

func (u *UnlockVault) unlock() {
	if u.button.Disabled() || u.password.Text == "" {
		return
	}

	password := u.password.Text
	u.password.SetText("")
	u.password.Disable()
	u.button.Disable()
	u.message.Hide()
	u.activity.Show()
	u.activity.Start()
	u.message.Importance = widget.MediumImportance
	if u.info.NeedsMigration {
		u.message.SetText("Migrating vault…")
	} else {
		u.message.SetText("Unlocking vault…")
	}
	u.message.Show()
	u.Refresh()

	go func() {
		key, err := u.store.LoadVaultKey(u.info.ID, password)
		if err != nil {
			var invalidPassword *age.NoIdentityMatchError
			if errors.As(err, &invalidPassword) {
				err = errors.New("the password is incorrect")
			}
			fyne.Do(func() { u.showError(err) })
			return
		}
		vault, err := u.store.LoadVault(u.info.ID, key)
		if err != nil {
			fyne.Do(func() { u.showError(err) })
			return
		}
		fyne.Do(func() {
			u.activity.Stop()
			u.activity.Hide()
			u.message.Hide()
			u.password.Enable()
			if u.onUnlocked != nil {
				u.onUnlocked(vault)
			}
		})
	}()
}

func (u *UnlockVault) showError(err error) {
	u.activity.Stop()
	u.activity.Hide()
	u.password.Enable()
	u.message.SetText(err.Error())
	u.message.Importance = widget.DangerImportance
	u.message.Show()
	u.Refresh()
}

func (u *UnlockVault) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(u.content)
}

func unlockVaultContent(info storage.VaultInfo, password *widget.Entry, unlock *widget.Button, activity *widget.Activity, message *widget.Label, back, addVault func()) *fyne.Container {
	heading := newText("Unlock "+info.Name, headingStyle)
	heading.Alignment = fyne.TextAlignCenter
	var navigation fyne.CanvasObject
	if back != nil {
		button := widget.NewButtonWithIcon("", icons.Back, back)
		button.Importance = widget.LowImportance
		navigation = button
	}
	more := newVaultOptions(addVault)
	header := container.NewBorder(nil, nil, navigation, more)
	badge := newVaultIcon(info.Appearance, vaultIconLarge)
	fields := container.NewVBox(
		container.New(layout.NewCustomPaddedLayout(24, 24, 0, 0), container.NewCenter(badge)),
		heading,
		password,
		container.NewStack(unlock),
		container.NewHBox(activity, message),
	)
	content := container.NewVBox(layout.NewSpacer(), fields, layout.NewSpacer())
	page := container.NewBorder(header, nil, nil, nil, container.NewVScroll(content))
	return container.New(layout.NewCustomPaddedLayout(24, 24, 24, 24), page)
}

// newVaultOptions provides the shared menu for opening the Add vault page.
func newVaultOptions(addVault func()) *widget.Button {
	more := widget.NewButtonWithIcon("", icons.More, nil)
	more.Importance = widget.LowImportance
	menu := fyne.NewMenu("", fyne.NewMenuItem("Add vault", addVault))
	more.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(menu, fyne.CurrentApp().Driver().CanvasForObject(more),
			fyne.NewPos(0, more.Size().Height), more)
	}
	return more
}
