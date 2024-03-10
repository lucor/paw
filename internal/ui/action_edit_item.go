// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/net/publicsuffix"
	"lucor.dev/paw/internal/paw"
)

func (a *app) makeEditItemView(fyneItem FyneItem) fyne.CanvasObject {

	item := fyneItem.Item()
	metadata := item.GetMetadata()

	ctx := context.TODO()
	content, editItem := fyneItem.Edit(ctx, a.vault.Key(), a.win)
	saveBtn := widget.NewButtonWithIcon("Save", theme.DocumentSaveIcon(), func() {
		metadata := editItem.GetMetadata()

		// TODO: update to use the built-in entry validation
		if metadata.Name == "" {
			d := dialog.NewInformation("", "The title cannot be emtpy", a.win)
			d.Show()
			return
		}

		var isNew bool
		if item.GetMetadata().IsEmpty() {
			isNew = true
		} else {
			metadata.Modified = time.Now()
		}

		if metadata.Type == paw.LoginItemType {
			login := editItem.(*paw.Login)
			if login.URL != "" {
				u, err := url.ParseRequestURI(login.URL)
				if err != nil {
					dialog.ShowError(fmt.Errorf("invalid URL: %s", err), a.win)
					return
				}
				tldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(u.Hostname())
				if err != nil {
					dialog.ShowError(fmt.Errorf("invalid URL: %s", err), a.win)
					return
				}
				metadata.Autofill = &paw.Autofill{
					URL:        u,
					MatchType:  paw.DomainMatchAutofill,
					TLDPlusOne: tldPlusOne,
				}
			}
		}

		if subtitler, ok := editItem.(paw.MetadataSubtitler); ok {
			metadata.Subtitle = subtitler.Subtitle()
		}

		if isNew && a.vault.HasItem(editItem) {
			msg := fmt.Sprintf("An item with the name %q already exists", metadata.Name)
			d := dialog.NewInformation("", msg, a.win)
			d.Show()
			return
		}

		// add item to vault and store into the storage
		a.vault.AddItem(editItem)
		err := a.storage.StoreItem(a.vault, editItem)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}

		// make sure key is removed from SSH agent to honour user's preference
		_ = a.removeSSHKeyFromAgent(item)

		if item.ID() != editItem.ID() {
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

		item = editItem
		fyneItem := NewFyneItem(item, a.config)
		a.refreshCurrentView()
		a.showItemView(fyneItem)
	})

	// elements should not be displayed on create but only on edit
	var metadataContent fyne.CanvasObject
	metadataContent = widget.NewLabel("")
	var deleteBtn fyne.CanvasObject
	if !metadata.IsEmpty() {
		metadataContent = ShowMetadata(metadata)
		button := widget.NewButtonWithIcon("Delete", theme.DeleteIcon(), func() {
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
					a.refreshCurrentView()
					a.showCurrentVaultView()
				}
			}, a.win)
			d.Show()
		})
		button.Importance = widget.DangerImportance
		deleteBtn = button
	}

	buttonContainer := container.NewBorder(nil, nil, deleteBtn, saveBtn, widget.NewLabel(""))
	bottom := container.NewBorder(nil, buttonContainer, nil, nil, metadataContent)

	return container.NewBorder(a.makeCancelHeaderButton(), bottom, nil, nil, content)
}
