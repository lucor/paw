package ui

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
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
		deleteBtn = container.NewMax(canvas.NewRectangle(color.NRGBA{0xd0, 0x17, 0x2d, 0xff}), button)
	}

	buttonContainer := container.NewBorder(nil, nil, deleteBtn, saveBtn, widget.NewLabel(""))
	bottom := container.NewBorder(nil, buttonContainer, nil, nil, metadataContent)

	return container.NewBorder(a.makeCancelHeaderButton(), bottom, nil, nil, content)
}
