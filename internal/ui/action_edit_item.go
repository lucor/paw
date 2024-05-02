// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

func (a *app) makeEditItemView(fyneItemWidget FyneItemWidget) fyne.CanvasObject {
	item := fyneItemWidget.Item()
	itemID := item.ID()
	isNew := item.GetMetadata().IsEmpty()

	itemEditWidget := newItemEditWidget(context.TODO(), a.vault.Key(), fyneItemWidget, a.win)
	itemEditWidget.OnSave = func() {
		// call OnSubmit to update the item with the latest data
		editItem, err := fyneItemWidget.OnSubmit()
		if err != nil {
			// handle validation error
			var verr *validationError
			if errors.As(err, &verr) {
				dialog.ShowError(verr, a.win)
				return
			}
			// if not a validation error something bad happened
			// show the message to the user and ask to report the error
			fErr := fmt.Errorf("something went wrong. Please report the following error: \n%s", err.Error())
			dialog.ShowError(fErr, a.win)
			return
		}
		updatedTime := time.Now().UTC()
		updatedMetadata := editItem.GetMetadata()

		if itemID != editItem.ID() && a.vault.HasItem(editItem) {
			msg := fmt.Sprintf("A %s item with the name %q already exists", updatedMetadata.Type.String(), updatedMetadata.Name)
			d := dialog.NewInformation("", msg, a.win)
			d.Show()
			return
		}

		if !isNew {
			updatedMetadata.Modified = updatedTime
		}

		// add item to vault and store into the storage
		a.vault.AddItem(editItem)
		err = a.storage.StoreItem(a.vault, editItem)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		// make sure key is removed from SSH agent to honour user's preference
		_ = a.removeSSHKeyFromAgent(item)

		if itemID != editItem.ID() {
			if !isNew {
				// item ID is changed, delete the old one
				a.vault.DeleteItem(item)
				err := a.storage.DeleteItem(a.vault, item)
				if err != nil {
					log.Printf("item rename: could not remove old item from storage: %s", err)
				}
			}
		}

		err = a.addSSHKeyToAgent(editItem)
		if err != nil {
			log.Println(err)
		}

		a.vault.Modified = updatedTime
		err = a.storage.StoreVault(a.vault)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		a.state.Modified = updatedTime
		err = a.storage.StoreAppState(a.state)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		a.refreshCurrentView()
		fyneItemWidget := NewFyneItemWidget(editItem, a.state.Preferences)
		a.showItemView(fyneItemWidget)
	}

	// elements should not be displayed on create but only on edit
	itemEditWidget.OnDelete = func() {
		editItem := fyneItemWidget.Item()
		msg := widget.NewLabel(fmt.Sprintf("Are you sure you want to delete %q?", item.String()))

		d := dialog.NewCustomConfirm("", "Delete", "Cancel", msg, func(b bool) {
			if b {
				a.vault.DeleteItem(editItem)
				err := a.storage.DeleteItem(a.vault, editItem)
				if err != nil {
					dialog.ShowError(err, a.win)
					return
				}
				err = a.removeSSHKeyFromAgent(item)
				if err != nil {
					log.Println(err)
				}

				now := time.Now().UTC()

				a.vault.Modified = now
				err = a.storage.StoreVault(a.vault)
				if err != nil {
					dialog.ShowError(err, a.win)
					return
				}

				a.state.Modified = now
				err = a.storage.StoreAppState(a.state)
				if err != nil {
					dialog.ShowError(err, a.win)
					return
				}

				a.refreshCurrentView()
				a.showCurrentVaultView()
			}
		}, a.win)
		d.SetConfirmImportance(widget.DangerImportance)
		d.Show()
	}

	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, itemEditWidget)
}
