// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"lucor.dev/paw/internal/haveibeenpwned"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// auditPasswordView returns a view to audit passwords
func (a *app) makeAuditPasswordView() fyne.CanvasObject {

	image := imageFromResource(icon.FactCheckOutlinedIconThemed)

	text := widget.NewLabel("Check Vault passwords against existing data breaches")
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter

	var content *fyne.Container

	auditBtn := widget.NewButtonWithIcon("Audit", icon.FactCheckOutlinedIconThemed, func() {

		ctx, cancel := context.WithCancel(context.Background())

		itemMetadata := a.vault.FilterItemMetadata(&paw.VaultFilterOptions{ItemType: paw.PasswordItemType | paw.LoginItemType})

		modalTitle := widget.NewLabel("Auditing items...")
		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, len(itemMetadata))
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling auditing, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		modalContent := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(modalContent, a.win.Canvas())

		var counter uint32
		pwendItems := []haveibeenpwned.Pwned{}

		sem := semaphore.NewWeighted(int64(maxWorkers))
		g := &errgroup.Group{}

		go func() {
			for _, meta := range itemMetadata {
				meta := meta

				err := sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					break
				}

				g.Go(func() error {
					defer sem.Release(1)

					item, err := a.storage.LoadItem(a.vault, meta)
					if err != nil {
						return err
					}

					isPwend, count, err := haveibeenpwned.Search(ctx, item)
					if err != nil {
						return err
					}
					if isPwend {
						pwendItems = append(pwendItems, haveibeenpwned.Pwned{Item: item, Count: count})
					}

					v := atomic.AddUint32(&counter, 1)
					progressBind.Set(float64(v))
					return nil
				})
			}

			defer modal.Hide()
			err := g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				dialog.ShowError(err, a.win)
				return
			}

			sort.Slice(pwendItems, func(i, j int) bool { return pwendItems[i].Count > pwendItems[j].Count })

			num := len(pwendItems)
			if num == 0 {
				image = imageFromResource(icon.CheckCircleOutlinedIconThemed)
				text.SetText("No password found in data breaches")
				content.Objects = []fyne.CanvasObject{container.NewVBox(image, text)}
				content.Refresh()
				return
			}

			image = imageFromResource(theme.WarningIcon())
			text.SetText("Passwords of the items below have been found in a data breaches and should not be used")
			list := widget.NewList(
				func() int {
					return len(pwendItems)
				},
				func() fyne.CanvasObject {
					return container.NewBorder(nil, nil, widget.NewIcon(icon.PasswordOutlinedIconThemed), widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil), widget.NewLabel("item label"))
				},
				func(lii widget.ListItemID, co fyne.CanvasObject) {
					v := pwendItems[lii]
					item := v.Item
					metadata := item.GetMetadata()
					fyneItem := NewFyneItem(v.Item, a.config)
					co.(*fyne.Container).Objects[0].(*widget.Label).SetText(fmt.Sprintf("%s (found %d times)", metadata.Name, v.Count))
					co.(*fyne.Container).Objects[1].(*widget.Icon).SetResource(fyneItem.Icon())
					co.(*fyne.Container).Objects[2].(*widget.Button).OnTapped = func() {
						a.showEditItemView(fyneItem)
					}
				},
			)
			list.OnSelected = func(id widget.ListItemID) {
				fyneItem := NewFyneItem(pwendItems[id].Item, a.config)
				a.showItemView(fyneItem)
			}

			content.Objects = []fyne.CanvasObject{container.NewBorder(container.NewVBox(image, text), nil, nil, nil, list)}
			content.Refresh()
		}()
		modal.Show()
	})
	auditBtn.Resize(auditBtn.MinSize())

	empty := widget.NewLabel("")
	content = container.NewVBox(image, text, container.NewGridWithColumns(3, empty, auditBtn, empty))
	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, content)
}
