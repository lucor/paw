// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"errors"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type masterPasswordView struct {
	view             fyne.CanvasObject
	current, confirm *widget.Entry
	password         *PasswordField
	submit, back     *widget.Button
	message          *widget.Label
	currentError     *widget.Label
	activity         *widget.Activity
	progress         *fyne.Container
	cancel           context.CancelFunc
}

func newMasterPasswordView(back func()) *masterPasswordView {
	v := &masterPasswordView{current: widget.NewPasswordEntry(), confirm: widget.NewPasswordEntry(), password: NewPasswordField(), message: newText("", bodyStyle), activity: widget.NewActivity()}
	v.message.Hide()
	v.currentError = newErrorText("Current master password is incorrect.")
	v.currentError.Hide()
	v.back = widget.NewButtonWithIcon("", icons.Back, back)
	v.back.Importance = widget.LowImportance
	v.submit = widget.NewButton("Change password", nil)
	v.submit.Importance = widget.HighImportance
	v.submit.Disable()
	v.confirm.Validator = func(value string) error {
		if value != v.password.Entry.Text {
			return errors.New("passwords do not match")
		}
		return nil
	}
	update := func(string) {
		if v.current.Text != "" && v.password.Entry.Text != "" && v.confirm.Text == v.password.Entry.Text {
			v.submit.Enable()
		} else {
			v.submit.Disable()
		}
		if v.confirm.Text == v.password.Entry.Text {
			v.confirm.SetValidationError(nil)
		}
	}
	v.current.OnChanged = func(value string) { v.currentError.Hide(); update(value) }
	v.confirm.OnChanged = update
	form := widget.NewForm(widget.NewFormItem("Current master password", container.NewVBox(v.current, v.currentError)), widget.NewFormItem("New master password", v.password), widget.NewFormItem("Confirm new password", v.confirm))
	form.Orientation = widget.Vertical
	passwordWasEmpty := true
	v.password.OnChanged = func(value string) {
		empty := value == ""
		if empty != passwordWasEmpty {
			form.Refresh()
			passwordWasEmpty = empty
		}
		update(value)
	}
	title := newText("Change master password", titleStyle)
	title.Truncation = fyne.TextTruncateEllipsis
	progress := container.NewCenter(v.activity)
	progress.Hide()
	v.progress = progress
	content := container.NewStack(container.NewVScroll(container.NewVBox(form, newText("Your vault will use the new master password the next time you unlock it.", captionStyle))), progress)
	v.view = newSecondaryPage(newPageHeader(v.back, title, nil), container.NewVBox(v.message, v.submit), content)
	return v
}

func (v *masterPasswordView) clear() {
	v.activity.Stop()
	if v.cancel != nil {
		v.cancel()
		v.cancel = nil
	}
	v.current.SetText("")
	v.password.Entry.SetText("")
	v.confirm.SetText("")
}

func (a *app) closeMasterPasswordView() {
	if a.passwordChange != nil {
		a.passwordChange.clear()
		a.passwordChange = nil
	}
}

func (a *app) showChangeMasterPassword() {
	if a.vault == nil || a.settingsVaultID != a.vaultID {
		return
	}
	a.closeMasterPasswordView()
	id := a.vaultID
	name, _ := a.vault.Name()
	infos, err := a.storage.Vaults()
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	info := storage.VaultInfo{ID: id, Name: name}
	for _, value := range infos {
		if value.ID == id {
			info = value
			break
		}
	}
	v := newMasterPasswordView(func() { a.closeMasterPasswordView(); a.showVaultSettings() })
	a.passwordChange = v
	v.submit.OnTapped = func() {
		if a.vault == nil || a.vaultID != id || v.current.Text == "" || v.password.Entry.Text == "" || v.confirm.Text != v.password.Entry.Text {
			return
		}
		current, next := v.current.Text, v.password.Entry.Text
		ctx, cancel := context.WithCancel(context.Background())
		v.cancel = cancel
		v.submit.Disable()
		v.back.Disable()
		v.current.Disable()
		v.password.Entry.Disable()
		v.confirm.Disable()
		v.message.Hide()
		v.currentError.Hide()
		v.progress.Show()
		v.activity.Start()
		v.submit.Hide()
		go func() {
			err := a.storage.ChangeMasterPassword(ctx, id, current, next)
			fyne.Do(func() {
				if err == nil && a.vault != nil && a.vaultID == id {
					a.main.Content = a.makeUnlockVaultView(info, nil)
					a.lockVault()
					a.main.Refresh()
					return
				}
				if ctx.Err() != nil || a.passwordChange != v || a.vaultID != id {
					return
				}
				v.back.Enable()
				v.activity.Stop()
				v.progress.Hide()
				v.submit.Show()
				v.current.Enable()
				v.password.Entry.Enable()
				v.confirm.Enable()
				v.submit.Enable()
				if errors.Is(err, storage.ErrIncorrectMasterPassword) {
					v.currentError.Show()
				} else {
					v.message.SetText("Could not change the master password. Try again.")
					v.message.Show()
				}
			})
		}()
	}
	a.win.SetContent(v.view)
}
