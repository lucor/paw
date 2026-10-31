// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// makeEditItemView builds the screen for editing or creating an item.
func (a *app) makeEditItemView(original *model.Item, isNew bool) fyne.CanvasObject {
	switch original.Metadata.Type {
	case paw.LoginItemType:
		return a.makeLoginEditView(original, isNew)
	case paw.PasswordItemType:
		return a.makePasswordEditView(original, isNew)
	case paw.NoteItemType:
		return a.makeNoteEditView(original, isNew)
	case paw.SSHKeyItemType:
		return a.makeSSHKeyEditView(original, isNew)
	}
	dialog.ShowError(fmt.Errorf("unsupported entry type"), a.win)
	return a.main
}

func (a *app) makeSSHKeyEditView(original *model.Item, isNew bool) fyne.CanvasObject {
	editor, err := NewSSHKeyEditor(original, isNew)
	if err != nil {
		dialog.ShowError(err, a.win)
		return a.main
	}
	editor.ConnectWindow(a.win)
	a.sshEditor = editor
	returnBack := func() {
		if isNew {
			a.showCurrentVaultView()
			return
		}
		a.showItemView(original)
	}
	editor.BackButton.OnTapped = func() { a.confirmEditorBack(editor.IsDirty(), returnBack) }
	editor.SaveButton.OnTapped = func() {
		item, err := editor.UpdatedItem()
		if err != nil {
			if !editor.HasValidationErrors() {
				dialog.ShowError(err, a.win)
			}
			return
		}
		save := func() { a.saveEditedItem(original, item, isNew) }
		if !isNew && editor.KeyChanged() {
			dialog.NewCustomConfirm("Replace SSH key?", "Replace", "Cancel", widget.NewLabel("The existing key will be replaced when you save."), func(replace bool) {
				if replace {
					save()
				}
			}, a.win).Show()
			return
		}
		save()
	}
	settings := paw.NewDefaultSettings()
	if a.state != nil && a.state.Settings != nil {
		settings = a.state.Settings
	}
	editor.GeneratePassphraseButton.OnTapped = func() {
		password := binding.NewString()
		password.AddListener(binding.NewDataListener(func() {
			value, _ := password.Get()
			editor.Passphrase.SetText(value)
			editor.PasswordSpec.Value = value
		}))
		NewPasswordGenerator(a.vault.Key(), settings.Password).ShowPasswordGenerator(password, editor.PasswordSpec, a.win)
	}
	return editor.View
}

func (a *app) makeLoginEditView(original *model.Item, isNew bool) fyne.CanvasObject {
	settings := paw.NewDefaultSettings()
	if a.state != nil && a.state.Settings != nil {
		settings = a.state.Settings
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.detailCancel = cancel
	editor, err := NewLoginEditor(ctx, original, settings.FaviconDownloader, isNew)
	if err != nil {
		cancel()
		a.detailCancel = nil
		dialog.ShowError(err, a.win)
		return a.main
	}
	returnBack := func() {
		if isNew {
			a.showCurrentVaultView()
			return
		}
		a.showItemView(original)
	}
	editor.BackButton.OnTapped = func() { a.confirmEditorBack(editor.IsDirty(), returnBack) }
	editor.SaveButton.OnTapped = func() {
		item, err := editor.UpdatedItem()
		if err != nil {
			if !editor.HasValidationErrors() {
				dialog.ShowError(err, a.win)
			}
			return
		}
		a.saveEditedItem(original, item, isNew)
	}
	editor.TOTPSettings.OnTapped = func() {
		showTOTPSettingsDialog(editor, a.win)
	}
	editor.GenerateButton.OnTapped = func() {
		password := binding.NewString()
		password.AddListener(binding.NewDataListener(func() {
			value, _ := password.Get()
			editor.Password.Entry.SetText(value)
		}))
		NewPasswordGenerator(a.vault.Key(), settings.Password).ShowPasswordGenerator(password, editor.PasswordSpec, a.win)
	}
	return editor.View
}

func (a *app) makePasswordEditView(original *model.Item, isNew bool) fyne.CanvasObject {
	editor, err := NewPasswordEditor(original, isNew)
	if err != nil {
		dialog.ShowError(err, a.win)
		return a.main
	}
	returnBack := func() {
		if isNew {
			a.showCurrentVaultView()
			return
		}
		a.showItemView(original)
	}
	editor.BackButton.OnTapped = func() { a.confirmEditorBack(editor.IsDirty(), returnBack) }
	editor.SaveButton.OnTapped = func() {
		item, err := editor.UpdatedItem()
		if err != nil {
			if !editor.HasValidationErrors() {
				dialog.ShowError(err, a.win)
			}
			return
		}
		a.saveEditedItem(original, item, isNew)
	}
	editor.GenerateButton.OnTapped = func() {
		settings := paw.NewDefaultSettings()
		if a.state != nil && a.state.Settings != nil {
			settings = a.state.Settings
		}
		password := binding.NewString()
		password.AddListener(binding.NewDataListener(func() {
			value, _ := password.Get()
			editor.Password.Entry.SetText(value)
		}))
		NewPasswordGenerator(a.vault.Key(), settings.Password).ShowPasswordGenerator(password, editor.PasswordSpec, a.win)
	}
	return editor.View
}

func (a *app) makeNoteEditView(original *model.Item, isNew bool) fyne.CanvasObject {
	editor, err := NewNoteEditor(original, isNew)
	if err != nil {
		dialog.ShowError(err, a.win)
		return a.main
	}
	returnBack := func() {
		if isNew {
			a.showCurrentVaultView()
			return
		}
		a.showItemView(original)
	}
	editor.BackButton.OnTapped = func() { a.confirmEditorBack(editor.IsDirty(), returnBack) }
	editor.SaveButton.OnTapped = func() {
		item, err := editor.UpdatedItem()
		if err != nil {
			if !editor.HasValidationErrors() {
				dialog.ShowError(err, a.win)
			}
			return
		}
		a.saveEditedItem(original, item, isNew)
	}
	return editor.View
}

func (a *app) confirmEditorBack(isDirty bool, returnBack func()) {
	if !isDirty {
		returnBack()
		return
	}
	dialog.NewCustomConfirm("Discard changes?", "Discard", "Keep editing", widget.NewLabel("Your changes will be lost."), func(discard bool) {
		if discard {
			returnBack()
		}
	}, a.win).Show()
}

func (a *app) saveEditedItem(original, item *model.Item, isNew bool) {
	if isNew {
		summaries, err := a.vault.FilterItems(item.Title, item.Metadata.Type)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		for _, summary := range summaries {
			if summary.Title == item.Title {
				dialog.ShowError(fmt.Errorf("an item with this name already exists"), a.win)
				return
			}
		}
	}
	now := uint64(time.Now().UTC().Unix())
	item.ModifiedAt = &now
	if !isNew {
		_ = a.removeSSHKeyFromAgent(original)
	}
	if err := a.storage.StoreItem(a.vault, item); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	if a.audit != nil {
		a.audit.MarkStale()
	}
	if err := a.addSSHKeyToAgent(item); err != nil {
		log.Println(err)
	}
	a.state.Modified = time.Unix(int64(now), 0).UTC()
	if err := a.storage.StoreAppState(a.state); err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.refreshCurrentView()
	a.showItemView(item)
}

// deleteItem confirms deletion and updates the existing vault navigation.
func (a *app) deleteItem(original *model.Item) {
	msg := widget.NewLabel(fmt.Sprintf("Are you sure you want to delete %q?", original.Title))
	d := dialog.NewCustomConfirm("", "Delete", "Cancel", msg, func(ok bool) {
		if !ok {
			return
		}
		if err := a.storage.DeleteItem(a.vault, original.ID); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if err := a.removeSSHKeyFromAgent(original); err != nil {
			log.Println(err)
		}
		a.state.Modified = time.Now().UTC()
		if err := a.storage.StoreAppState(a.state); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.refreshCurrentView()
		a.showCurrentVaultView()
	}, a.win)
	d.SetConfirmImportance(widget.DangerImportance)
	d.Show()
}
