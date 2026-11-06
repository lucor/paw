// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	json "encoding/json/v2"
	"fmt"
	"log"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func newItemHistoryView(history model.History, back func(), open func(int, model.HistoryEntry)) fyne.CanvasObject {
	displayed := make([]model.HistoryEntry, len(history.Entries))
	for index := range displayed {
		displayed[index] = history.Entries[len(history.Entries)-1-index]
	}
	var list *widget.List
	list = widget.NewList(func() int { return len(displayed) }, func() fyne.CanvasObject {
		return container.NewStack(newNavigationRow(" ", navigationRowIcon(theme.HistoryIcon()), nil))
	}, func(id widget.ListItemID, object fyne.CanvasObject) {
		entry := displayed[id]
		row := newNavigationRow(formatDetailTimestamp(entry.CapturedAt), navigationRowIcon(theme.HistoryIcon()), func() { list.Select(id) })
		cell := object.(*fyne.Container)
		cell.Objects[0] = row
		cell.Refresh()
	})
	list.OnSelected = func(id widget.ListItemID) {
		entry := displayed[id]
		list.UnselectAll()
		open(len(history.Entries)-1-id, entry)
	}
	backButton := newIconButton("Back", icons.Back, back)
	backButton.Importance = widget.LowImportance
	return newSecondaryPage(newPageHeader(backButton, newText("History", titleStyle), nil), nil, list)
}

func (a *app) showItemHistory(itemID string, itemBack func()) {
	if a.vault == nil || a.storage == nil {
		dialog.ShowError(fmt.Errorf("vault is no longer available"), a.win)
		a.returnFromItemHistory(itemBack)
		return
	}
	item, err := a.storage.LoadItem(a.vault, itemID)
	if err != nil {
		dialog.ShowError(err, a.win)
		a.returnFromItemHistory(itemBack)
		return
	}
	history, err := item.History()
	if err != nil {
		dialog.ShowError(err, a.win)
		a.returnFromItemHistory(itemBack)
		return
	}
	if len(history.Entries) == 0 {
		dialog.ShowError(fmt.Errorf("item has no history"), a.win)
		a.showCurrentItemDetail(item, itemBack)
		return
	}
	a.closeItemDetail()
	view := newItemHistoryView(history, func() {
		a.showCurrentItemAfterHistory(itemID, itemBack)
	}, func(index int, entry model.HistoryEntry) {
		a.showHistoricalItemDetail(itemID, index, entry, itemBack)
	})
	a.win.SetContent(view)
}

func (a *app) showCurrentItemAfterHistory(itemID string, itemBack func()) {
	if a.vault == nil || a.storage == nil {
		a.returnFromItemHistory(itemBack)
		return
	}
	item, err := a.storage.LoadItem(a.vault, itemID)
	if err != nil {
		dialog.ShowError(err, a.win)
		a.returnFromItemHistory(itemBack)
		return
	}
	a.showCurrentItemDetail(item, itemBack)
}

func (a *app) returnFromItemHistory(itemBack func()) {
	if itemBack != nil {
		itemBack()
		return
	}
	a.showCurrentVaultView()
}

func (a *app) showCurrentItemDetail(item *model.Item, itemBack func()) {
	if itemBack == nil {
		a.showItemView(item)
		return
	}
	a.showItemViewWithBack(item, itemBack)
}

func (a *app) showHistoricalItemDetail(itemID string, index int, expected model.HistoryEntry, itemBack func()) {
	data, err := json.Marshal(expected.Item, json.Deterministic(true))
	if err != nil {
		dialog.ShowError(err, a.win)
		a.showItemHistory(itemID, itemBack)
		return
	}
	var item model.Item
	if err := json.Unmarshal(data, &item); err != nil {
		dialog.ShowError(err, a.win)
		a.showItemHistory(itemID, itemBack)
		return
	}
	a.closeItemDetail()
	ctx, cancel := context.WithCancel(context.Background())
	a.detailCancel = cancel
	view, clear, err := newHistoricalEntryDetailWithClear(ctx, &item, a.win, expected.CapturedAt, func() {
		a.showItemHistory(itemID, itemBack)
	}, func() {
		a.confirmItemHistoryRestore(itemID, index, expected, itemBack)
	})
	if err != nil {
		a.closeItemDetail()
		dialog.ShowError(err, a.win)
		a.showItemHistory(itemID, itemBack)
		return
	}
	a.detailClear = clear
	a.win.SetContent(view)
}

func (a *app) confirmItemHistoryRestore(itemID string, index int, expected model.HistoryEntry, itemBack func()) {
	message := newText("This will replace the current item with the selected version. The current version will be saved in History, so you can restore it later.", bodyStyle)
	dialog.NewCustomConfirm("Restore previous version?", "Restore", "Cancel", message, func(restore bool) {
		if restore {
			a.restoreItemHistory(itemID, index, expected, itemBack)
		}
	}, a.win).Show()
}

func (a *app) restoreItemHistory(itemID string, index int, expected model.HistoryEntry, itemBack func()) {
	current, err := a.storage.LoadItem(a.vault, itemID)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	now := uint64(time.Now().UTC().Unix())
	restored, changed, err := a.storage.RestoreItem(a.vault, itemID, index, &expected, now)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	if changed {
		if err := a.removeSSHKeyFromAgent(current); err != nil {
			log.Println(err)
		}
		if a.audit != nil {
			a.audit.MarkStale()
		}
		if a.state != nil {
			a.state.Modified = time.Unix(int64(now), 0).UTC()
			if err := a.storage.StoreAppState(a.state); err != nil {
				dialog.ShowError(err, a.win)
			}
		}
		if err := a.addSSHKeyToAgent(restored); err != nil {
			log.Println(err)
		}
	}
	a.refreshCurrentView()
	a.showItemViewWithBack(restored, itemBack)
}
