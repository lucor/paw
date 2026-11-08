// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

func (a *app) attachmentDetailActions(item *model.Item, back func()) (*attachmentDetailActions, error) {
	if item.Metadata.Type != model.NoteItemType && item.Metadata.Type != model.LoginItemType {
		return nil, nil
	}
	files, err := item.Files()
	if err != nil {
		return nil, fmt.Errorf("cannot read attachment metadata: %w", err)
	}
	if a.storage == nil || a.vault == nil {
		return &attachmentDetailActions{files: files}, nil
	}
	actions := &attachmentDetailActions{files: files}
	actions.add = func() {
		chooseAttachmentFile(a.win, func(reader fyne.URIReadCloser) {
			name := reader.URI().Name()
			if name == "" {
				_ = reader.Close()
				dialog.ShowError(fmt.Errorf("selected file has no name"), a.win)
				return
			}
			vault := a.vault
			a.beginAttachmentTransfer("Adding attachment…")
			go func() {
				_, err := a.storage.AddAttachment(vault, item.ID, name, reader)
				_ = reader.Close()
				fyne.Do(func() {
					if a.finishAttachmentTransfer() {
						return
					}
					if err != nil {
						a.showItemViewWithBack(item, back)
						dialog.ShowError(err, a.win)
						return
					}
					a.attachmentChanged(item.ID, back)
				})
			}()
		})
	}
	actions.save = func(file cxf.File) {
		chooseAttachmentDestination(a.win, file, func(path string) {
			vault := a.vault
			a.beginAttachmentTransfer("Saving attachment…")
			go func() {
				err := a.storage.SaveAttachment(vault, item.ID, file.ID, path)
				fyne.Do(func() {
					if a.finishAttachmentTransfer() {
						return
					}
					a.showItemViewWithBack(item, back)
					if err != nil {
						dialog.ShowError(err, a.win)
						return
					}
					fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Paw", Content: file.Name + " saved"})
				})
			}()
		})
	}
	actions.remove = func(file cxf.File) {
		if err := a.storage.RemoveAttachment(a.vault, item.ID, file.ID); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.attachmentChanged(item.ID, back)
	}
	return actions, nil
}

func (a *app) beginAttachmentTransfer(title string) {
	a.attachmentBusy = true
	a.pendingLock = false
	a.pendingLockAllVaults = false
	a.win.SetMainMenu(nil)
	a.win.SetContent(container.NewVBox(newText(title, bodyStyle), widget.NewProgressBarInfinite()))
}

func (a *app) finishAttachmentTransfer() bool {
	a.attachmentBusy = false
	if a.pendingLockAllVaults {
		a.pendingLockAllVaults = false
		a.pendingLock = false
		a.lockAllVaults()
		a.win.SetMainMenu(a.makeMainMenu())
		return true
	}
	if a.pendingLock {
		a.pendingLock = false
		if a.vault != nil {
			name, _ := a.vault.Name()
			a.main.Content = a.makeUnlockVaultView(model.VaultInfo{ID: a.vaultID, Name: name}, nil, nil)
			a.main.Refresh()
		}
		a.lockVault()
		a.win.SetMainMenu(a.makeMainMenu())
		return true
	}
	a.win.SetMainMenu(a.makeMainMenu())
	return false
}

func (a *app) attachmentChanged(itemID string, back func()) {
	item, err := a.storage.LoadItem(a.vault, itemID)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	if a.audit != nil {
		a.audit.MarkStale()
	}
	if a.state != nil {
		a.state.Modified = time.Now().UTC()
		if err := a.storage.StoreAppState(a.state); err != nil {
			dialog.ShowError(err, a.win)
		}
	}
	a.refreshCurrentView()
	a.showItemViewWithBack(item, back)
}
