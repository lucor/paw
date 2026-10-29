// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"go.lucor.dev/paw/internal/model"
)

// makeShowItemView displays an item's full credentials and metadata.
func (a *app) makeShowItemView(item *model.Item) fyne.CanvasObject {
	return a.makeShowItemViewWithBack(item, nil)
}

// makeShowItemViewWithBack displays an item's detail with an optional return destination.
func (a *app) makeShowItemViewWithBack(item *model.Item, back func()) fyne.CanvasObject {
	a.closeItemDetail()
	ctx, cancel := context.WithCancel(context.Background())
	a.detailCancel = cancel
	if back == nil {
		back = a.showCurrentVaultView
		if a.audit != nil {
			back = func() { a.closeItemDetail(); a.audit.DetailBack() }
		}
	}
	view, err := NewEntryDetail(ctx, item, a.win, back, func() { a.showEditItemView(item, false) }, func() { a.deleteItem(item) })
	if err == nil {
		return view
	}
	a.closeItemDetail()
	dialog.ShowError(err, a.win)
	return a.main
}

// showCodesItem opens a login detail and returns to the existing Codes list.
func (a *app) showCodesItem(item *model.Item) {
	back := func() {
		a.closeItemDetail()
		a.win.SetContent(a.main)
	}
	a.win.SetContent(a.makeShowItemViewWithBack(item, back))
}

// openCodeEntry loads a login from the active vault and opens its detail view.
func (a *app) openCodeEntry(itemID string) {
	if a.vault == nil {
		return
	}
	item, err := a.storage.LoadItem(a.vault, itemID)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	a.showCodesItem(item)
}
