// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"go.lucor.dev/paw/internal/model"
)

// importFromFile reads CXF items and adds them to the current vault.
func (a *app) importFromFile() {
	dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		var items []*model.Item
		if err := json.NewDecoder(reader).Decode(&items); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		stored := make([]string, 0, len(items))
		for _, item := range items {
			if item == nil {
				err = fmt.Errorf("import contains an empty item")
				break
			}
			if err = a.storage.StoreItem(a.vault, item); err != nil {
				break
			}
			stored = append(stored, item.ID)
		}
		if err != nil {
			for _, id := range stored {
				_ = a.storage.DeleteItem(a.vault, id)
			}
			dialog.ShowError(err, a.win)
			return
		}
		a.state.Modified = time.Now().UTC()
		if err := a.storage.StoreAppState(a.state); err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		a.refreshCurrentView()
		a.showCurrentVaultView()
	}, a.win).Show()
}
