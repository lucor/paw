// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"go.lucor.dev/paw/internal/model"
)

// exportToFile saves all full vault items as CXF JSON.
func (a *app) exportToFile() {
	d := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		if writer == nil {
			return
		}
		defer writer.Close()
		summaries, err := a.vault.FilterItems("", 0)
		if err != nil {
			dialog.ShowError(err, a.win)
			return
		}
		items := make([]*model.Item, 0, len(summaries))
		for _, summary := range summaries {
			item, err := a.storage.LoadItem(a.vault, summary.ID)
			if err != nil {
				dialog.ShowError(err, a.win)
				return
			}
			items = append(items, item)
		}
		if err := json.NewEncoder(writer).Encode(items); err != nil {
			dialog.ShowError(err, a.win)
		}
	}, a.win)
	name, _ := a.vault.Name()
	d.SetFileName(fmt.Sprintf("%s.cxf.json", name))
	d.Show()
}
