// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"fmt"

	"filippo.io/age"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// UnlockVault displays and handles unlocking one stored vault.
type UnlockVault struct {
	widget.BaseWidget
	info              model.VaultInfo
	store             storage.VaultStorage
	onUnlocked        func(*model.Vault)
	password          *widget.Entry
	passwordError     *widget.Label
	incorrectPassword bool
	button            *widget.Button
	activity          *widget.Activity
	message           *widget.Label
	content           fyne.CanvasObject
	unlockDone        chan struct{}
}

const incorrectMasterPasswordMessage = "Incorrect master password. Try again."

// NewUnlockVault creates a vault unlock view with navigation to Add vault.
func NewUnlockVault(info model.VaultInfo, store storage.VaultStorage, onUnlocked func(*model.Vault), back, addVault, chooseAnother func()) *UnlockVault {
	u := &UnlockVault{info: info, store: store, onUnlocked: onUnlocked}
	u.password = widget.NewPasswordEntry()
	u.password.Validator = func(value string) error {
		if u.incorrectPassword {
			return fmt.Errorf("%s", incorrectMasterPasswordMessage)
		}
		if value == "" {
			return errors.New("master password is required")
		}
		return nil
	}
	u.passwordError = newErrorText(incorrectMasterPasswordMessage)
	u.passwordError.Hide()
	u.button = widget.NewButton("Unlock", u.unlock)
	u.button.Importance = widget.HighImportance
	u.button.Disable()
	u.activity = widget.NewActivity()
	u.activity.Hide()
	u.password.OnChanged = func(value string) {
		if u.incorrectPassword {
			u.incorrectPassword = false
			u.password.SetValidationError(nil)
		}
		if u.passwordError.Visible() {
			u.passwordError.Hide()
			u.Refresh()
		}
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
	back = u.clearPasswordWhenLeaving(back)
	addVault = u.clearPasswordWhenLeaving(addVault)
	chooseAnother = u.clearPasswordWhenLeaving(chooseAnother)
	u.content = unlockVaultContent(info, u.password, u.passwordError, u.button, u.activity, u.message, back, addVault, chooseAnother)
	u.ExtendBaseWidget(u)
	return u
}

func (u *UnlockVault) clearPasswordWhenLeaving(action func()) func() {
	if action == nil {
		return nil
	}
	return func() {
		u.ClearPassword()
		action()
	}
}

// ClearPassword clears sensitive input when this view is left.
func (u *UnlockVault) ClearPassword() {
	u.incorrectPassword = false
	u.password.SetText("")
	u.password.SetValidationError(nil)
	u.passwordError.Hide()
	u.message.Hide()
	u.button.Disable()
	u.Refresh()
}

func (u *UnlockVault) unlock() {
	if u.button.Disabled() || u.password.Text == "" {
		return
	}

	password := u.password.Text
	u.password.Disable()
	u.button.Disable()
	u.passwordError.Hide()
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

	u.unlockDone = make(chan struct{})
	done := u.unlockDone
	go func() {
		defer close(done)
		key, err := u.store.LoadVaultKey(u.info.ID, password)
		if err != nil {
			var invalidPassword *age.NoIdentityMatchError
			if errors.As(err, &invalidPassword) {
				fyne.DoAndWait(u.showIncorrectPassword)
				return
			}
			fyne.DoAndWait(func() { u.showError(err) })
			return
		}
		vault, err := u.store.LoadVault(u.info.ID, key)
		if err != nil {
			fyne.DoAndWait(func() { u.showError(err) })
			return
		}
		fyne.DoAndWait(func() {
			u.activity.Stop()
			u.activity.Hide()
			u.ClearPassword()
			if u.onUnlocked != nil {
				u.onUnlocked(vault)
			}
		})
	}()
}

func (u *UnlockVault) showIncorrectPassword() {
	u.activity.Stop()
	u.activity.Hide()
	u.message.Hide()
	u.password.Enable()
	u.incorrectPassword = true
	u.password.SetValidationError(fmt.Errorf("%s", incorrectMasterPasswordMessage))
	u.passwordError.Show()
	if u.password.Text == "" {
		u.button.Disable()
	} else {
		u.button.Enable()
	}
	if canvas := fyne.CurrentApp().Driver().CanvasForObject(u.password); canvas != nil {
		canvas.Focus(u.password)
	}
	u.Refresh()
}

func (u *UnlockVault) showError(err error) {
	u.activity.Stop()
	u.activity.Hide()
	u.incorrectPassword = false
	u.password.SetText("")
	u.password.SetValidationError(nil)
	u.password.Enable()
	u.message.SetText(err.Error())
	u.message.Importance = widget.DangerImportance
	u.message.Show()
	u.Refresh()
}

func (u *UnlockVault) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(u.content)
}

func unlockVaultContent(info model.VaultInfo, password *widget.Entry, passwordError *widget.Label, unlock *widget.Button, activity *widget.Activity, message *widget.Label, back, addVault, chooseAnother func()) fyne.CanvasObject {
	heading := newText("Unlock "+info.Name, headingStyle)
	heading.Alignment = fyne.TextAlignCenter
	var navigation fyne.CanvasObject
	if back != nil {
		button := widget.NewButtonWithIcon("", icons.Back, back)
		button.Importance = widget.LowImportance
		navigation = button
	}
	more := newVaultOptions(addVault)
	header := newPageHeader(navigation, nil, more)
	badge := newVaultIcon(info.Appearance, vaultIconLarge)
	var chooseAnotherAction fyne.CanvasObject
	if chooseAnother != nil {
		button := widget.NewButton("Choose another vault", chooseAnother)
		button.Importance = widget.LowImportance
		chooseAnotherAction = container.NewCenter(button)
	}
	fields := []fyne.CanvasObject{
		container.New(layout.NewCustomPaddedLayout(24, 24, 0, 0), container.NewCenter(badge)),
		heading,
		container.NewVBox(newText("Master password", captionStyle), password, container.NewThemeOverride(passwordError, compactTextTheme{Theme: theme.Current()})),
		container.NewStack(unlock),
	}
	if chooseAnotherAction != nil {
		fields = append(fields, chooseAnotherAction)
	}
	fields = append(fields, container.NewHBox(activity, message))
	content := container.NewVBox(fields...)
	return newSecondaryPage(header, nil, container.NewVScroll(content))
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
