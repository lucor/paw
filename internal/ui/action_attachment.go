// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
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
			defer reader.Close()
			name := reader.URI().Name()
			if name == "" {
				dialog.ShowError(fmt.Errorf("selected file has no name"), a.win)
				return
			}
			if _, err := a.storage.AddAttachment(a.vault, item.ID, name, reader); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			a.attachmentChanged(item.ID, back)
		})
	}
	actions.save = func(file cxf.File) {
		chooseAttachmentDestination(a.win, file, func(path string) {
			if err := a.storage.SaveAttachment(a.vault, item.ID, file.ID, path); err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Paw", Content: file.Name + " saved"})
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
