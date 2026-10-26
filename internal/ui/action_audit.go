// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/haveibeenpwned"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/paw"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
)

// auditPasswordView returns a view to audit passwords
func (a *app) makeAuditPasswordView() fyne.CanvasObject {
	auditBtn := widget.NewButtonWithIcon("Audit", icon.ChecklistOutlinedIconThemed, func() {
		a.startPasswordAudit()
	})

	image := imageFromResource(icon.ChecklistOutlinedIconThemed)
	text := widget.NewLabel("Check Vault passwords against existing data breaches")
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter
	c := container.NewBorder(container.NewVBox(image, text, auditBtn), nil, nil, nil, nil)
	return container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, c)
}

// startPasswordAudit collects password items and starts the audit.
func (a *app) startPasswordAudit() {
	itemMetadata, err := a.vault.FilterItems("", paw.PasswordItemType|paw.LoginItemType)
	if err != nil {
		dialog.ShowError(err, a.win)
		return
	}
	ctx, cancel := context.WithCancel(context.Background())

	// Create UI components on main thread
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
	modal.Show()

	// Start audit in background
	go a.performPasswordAudit(ctx, itemMetadata, progressBind, modal)
}

// performPasswordAudit checks each password item and updates progress.
func (a *app) performPasswordAudit(ctx context.Context, itemMetadata []cxf.Item, progressBind binding.Float, modal *widget.PopUp) {
	defer func() {
		fyne.Do(func() {
			modal.Hide()
		})
	}()

	var counter uint32
	var mu sync.Mutex
	pwendItems := []haveibeenpwned.Pwned{}

	sem := semaphore.NewWeighted(int64(maxWorkers))
	g := &errgroup.Group{}

	for _, summary := range itemMetadata {
		summary := summary

		err := sem.Acquire(ctx, 1)
		if err != nil {
			break
		}

		g.Go(func() error {
			defer sem.Release(1)

			item, err := a.storage.LoadItem(a.vault, summary.ID)
			if err != nil {
				return err
			}

			password := ""
			for _, raw := range item.Credentials {
				var credential cxf.BasicAuth
				if json.Unmarshal(raw, &credential) == nil && credential.Password != nil {
					password = credential.Password.Value
					break
				}
			}
			isPwend, count, err := haveibeenpwned.Search(ctx, password)
			if err != nil {
				return err
			}
			if isPwend {
				mu.Lock()
				pwendItems = append(pwendItems, haveibeenpwned.Pwned{Item: item, Count: count})
				mu.Unlock()
			}

			v := atomic.AddUint32(&counter, 1)
			fyne.Do(func() {
				progressBind.Set(float64(v))
			})
			return nil
		})
	}

	err := g.Wait()
	if err != nil || errors.Is(ctx.Err(), context.Canceled) {
		if err != nil {
			fyne.Do(func() {
				dialog.ShowError(err, a.win)
			})
		}
		return
	}

	mu.Lock()
	sort.Slice(pwendItems, func(i, j int) bool { return pwendItems[i].Count > pwendItems[j].Count })
	results := make([]haveibeenpwned.Pwned, len(pwendItems))
	copy(results, pwendItems)
	mu.Unlock()

	fyne.Do(func() {
		a.showAuditResults(results)
	})
}

// showAuditResults displays the passwords found in breach data.
func (a *app) showAuditResults(pwendItems []haveibeenpwned.Pwned) {
	if len(pwendItems) == 0 {
		image := imageFromResource(icon.CircleCheckOutlinedIconThemed)
		text := widget.NewLabel("No password found in data breaches")
		text.Wrapping = fyne.TextWrapWord
		text.Alignment = fyne.TextAlignCenter
		a.win.SetContent(container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, container.NewVBox(image, text)))
		return
	}

	image := imageFromResource(theme.WarningIcon())
	text := widget.NewLabel("Passwords of the items below have been found in a data breaches and should not be used")
	text.Wrapping = fyne.TextWrapWord
	text.Alignment = fyne.TextAlignCenter

	list := widget.NewList(
		func() int {
			return len(pwendItems)
		},
		func() fyne.CanvasObject {
			label := widget.NewLabel("item label")
			icon := widget.NewIcon(icon.PasswordOutlinedIconThemed)
			button := widget.NewButtonWithIcon("", theme.DocumentCreateIcon(), nil)
			return container.NewBorder(nil, nil, label, button, icon)
		},
		func(lii widget.ListItemID, co fyne.CanvasObject) {
			if lii >= len(pwendItems) {
				return
			}
			v := pwendItems[lii]
			metadata := v.Item
			fyneItemWidget := NewFyneItemWidget(v.Item, a.state.Preferences)

			border := co.(*fyne.Container)
			label := border.Objects[0].(*widget.Label)
			button := border.Objects[1].(*widget.Button)
			icon := border.Objects[2].(*widget.Icon)

			label.SetText(fmt.Sprintf("%s (found %d times)", metadata.Title, v.Count))
			icon.SetResource(fyneItemWidget.Icon())
			button.OnTapped = func() {
				a.showEditItemView(fyneItemWidget)
			}
		},
	)

	list.OnSelected = func(id widget.ListItemID) {
		if id >= len(pwendItems) {
			return
		}
		fyneItemWidget := NewFyneItemWidget(pwendItems[id].Item, a.state.Preferences)
		a.showItemView(fyneItemWidget)
	}

	c := container.NewBorder(container.NewVBox(image, text), nil, nil, nil, list)
	a.win.SetContent(container.NewBorder(a.makeCancelHeaderButton(), nil, nil, nil, c))
}
