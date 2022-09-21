package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
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

func (a *app) exportToFile() {
	d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, e error) {
		if e != nil {
			dialog.NewError(e, a.win).Show()
			return
		}
		ctx, cancel := context.WithCancel(context.Background())

		var counter uint32

		modalTitle := widget.NewLabel("Exporting items...")

		progressBind := binding.NewFloat()
		progressbar := widget.NewProgressBarWithData(progressBind)
		progressbar.TextFormatter = func() string {
			v, _ := progressBind.Get()
			return fmt.Sprintf("%.0f of %d", v, a.vault.Size())
		}

		var cancelButton *widget.Button
		cancelButton = widget.NewButton("Cancel", func() {
			modalTitle.SetText("Cancelling export, please wait...")
			progressbar.Hide()
			cancelButton.Disable()
			cancel()
		})

		c := container.NewBorder(modalTitle, nil, nil, nil, container.NewCenter(container.NewVBox(progressbar, cancelButton)))
		modal := widget.NewModalPopUp(c, a.win.Canvas())

		go func() {
			if uc == nil {
				// file open dialog has been cancelled
				modal.Hide()
				return
			}
			defer uc.Close()

			sem := semaphore.NewWeighted(int64(maxWorkers))
			g := &errgroup.Group{}

			mu := &sync.Mutex{}
			data := map[string][]paw.Item{}

			a.vault.Range(func(id string, meta *paw.Metadata) bool {
				err := sem.Acquire(ctx, 1)
				if err != nil {
					cancel()
					return false
				}

				g.Go(func() error {
					defer sem.Release(1)
					item, err := a.storage.LoadItem(a.vault, meta)
					if err != nil {
						return err
					}

					itemType := item.GetMetadata().Type.String()

					mu.Lock()
					data[itemType] = append(data[itemType], item)
					mu.Unlock()

					v := atomic.AddUint32(&counter, 1)
					progressBind.Set(float64(v))
					return nil
				})
				return true
			})

			defer modal.Hide()
			err := g.Wait()
			if err != nil || errors.Is(ctx.Err(), context.Canceled) {
				dialog.ShowError(err, a.win)
				return
			}

			err = json.NewEncoder(uc).Encode(data)
			if err != nil {
				dialog.ShowError(err, a.win)
			}
		}()
		modal.Show()
	}, a.win)
	d.SetFileName(fmt.Sprintf("%s.paw.json", a.vault.Name))
	d.Show()
}
