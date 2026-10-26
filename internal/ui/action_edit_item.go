// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// makeEditItemView builds the screen for editing or creating an item.
func (a *app) makeEditItemView(itemWidget FyneItemWidget) fyne.CanvasObject {
	original := itemWidget.Item()
	isNew := itemWidget.IsNew()
	editor := newItemEditWidget(context.TODO(), a.vault.Key(), itemWidget, a.win)
	editor.OnSave = func() {
		item, err := itemWidget.OnSubmit()
		if err != nil {
			var validationErr *validationError
			if errors.As(err, &validationErr) {
				dialog.ShowError(validationErr, a.win)
			} else {
				dialog.ShowError(fmt.Errorf("could not save item: %w", err), a.win)
			}
			return
		}
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
		if err := a.addSSHKeyToAgent(item); err != nil {
			log.Println(err)
		}
		a.state.Modified = time.Unix(int64(now), 0).UTC()
		if err := a.storage.StoreAppState(a.state); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.refreshCurrentView()
		a.showItemView(NewFyneItemWidget(item, a.state.Preferences))
	}
	editor.OnDelete = func() {
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
	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, editor)
}
