package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"lucor.dev/paw/internal/paw"
)

func (a *app) importFromFile() {
	d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, e error) {

		ctx, cancel := context.WithCancel(context.Background())

		data := paw.Imported{}
		var counter uint32

		modalTitle := widget.NewLabel("Importing items...")

		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, len(data.Items))
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling import, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		c := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(c, a.win.Canvas())

		rollback := func(vault *paw.Vault, items []paw.Item) {
			for _, item := range items {
				a.storage.DeleteItem(a.vault, item)
				a.vault.DeleteItem(item)
			}
		}

		go func() {
			if uc == nil {
				// file open dialog has been cancelled
				modal.Hide()
				return
			}
			defer uc.Close()
			// Decode the JSON input file
			err := json.NewDecoder(uc).Decode(&data)
			if err != nil {
				modal.Hide()
				dialog.ShowError(err, a.win)
				return
			}

			sem := semaphore.NewWeighted(int64(maxWorkers))
			g := &errgroup.Group{}

			processed := []paw.Item{}
			// TODO: handle if an item with same name and type already exists
			for _, item := range data.Items {
				item := item

				err = sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					break
				}

				g.Go(func() error {
					defer sem.Release(1)
					err := a.storage.StoreItem(a.vault, item)
					if err != nil {
						return err
					}
					processed = append(processed, item)
					v := atomic.AddUint32(&counter, 1)
					progressBind.Set(float64(v))
					return nil
				})
			}

			defer modal.Hide()
			err = g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				rollback(a.vault, processed)
				dialog.ShowError(err, a.win)
				return
			}

			for _, item := range processed {
				a.vault.AddItem(item)
			}
			err = a.storage.StoreVault(a.vault)
			if err != nil {
				rollback(a.vault, processed)
				dialog.ShowError(err, a.win)
				return
			}
			a.refreshCurrentView()
			a.showCurrentVaultView()
		}()

		modal.Show()

	}, a.win)
	d.Show()
}
