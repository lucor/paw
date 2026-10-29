// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

type restoreFlow struct {
	window   fyne.Window
	store    storage.Storage
	back     func()
	restored func(*model.Vault)
}

// ShowRestoreBackup opens single-page navigation for selecting and restoring a backup.
func ShowRestoreBackup(window fyne.Window, store storage.Storage, restored func(*model.Vault)) {
	previous := window.Content()
	f := &restoreFlow{window: window, store: store, restored: restored, back: func() { window.SetContent(previous) }}
	f.selectFile("")
}

func (f *restoreFlow) page(title string, back func(), content fyne.CanvasObject) {
	f.window.SetContent(auditPage(title, back, content))
}

func (f *restoreFlow) selectFile(message string) {
	choose := widget.NewButton("Select backup file", func() {
		dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				f.selectFile(err.Error())
				return
			}
			if reader == nil {
				return
			}
			f.decode(reader, "")
		}, f.window).Show()
	})
	choose.Importance = widget.HighImportance
	feedback := newErrorText(message)
	if message == "" {
		feedback.Hide()
	}
	f.page("Restore from backup", f.back, container.NewVBox(newText("Restore a Paw export (.cxf or .cxf.age).", bodyStyle), feedback, choose))
}

func (f *restoreFlow) busy(title string, cancel func()) {
	f.page(title, cancel, container.NewVBox(newText(title, bodyStyle), widget.NewProgressBarInfinite(), widget.NewButton("Cancel", cancel)))
}

func (f *restoreFlow) decode(reader fyne.URIReadCloser, password string) {
	uri := reader.URI()
	ctx, cancel := context.WithCancel(context.Background())
	f.busy("Checking backup", func() { cancel(); f.selectFile("") })
	go func() {
		backup, err := storage.ReadPawBackup(ctx, reader, password)
		closeErr := reader.Close()
		if err == nil {
			err = closeErr
		}
		fyne.Do(func() {
			wasCancelled := ctx.Err() != nil
			cancel()
			if wasCancelled {
				return
			}
			if errors.Is(err, storage.ErrBackupPasswordRequired) {
				f.unlock(uri, "")
				return
			}
			if err != nil {
				if password != "" && !errors.Is(err, storage.ErrUnsupportedBackup) {
					f.unlock(uri, restoreErrorMessage(err))
					return
				}
				f.selectFile(restoreErrorMessage(err))
				return
			}
			collision, err := storage.BackupCollision(f.store, backup)
			if err != nil {
				f.selectFile(err.Error())
				return
			}
			f.review(backup, collision, "")
		})
	}()
}

func restoreErrorMessage(err error) string {
	if errors.Is(err, storage.ErrUnsupportedBackup) {
		return storage.UnsupportedBackupMessage
	}
	return err.Error()
}

func (f *restoreFlow) unlock(uri fyne.URI, message string) {
	entry := widget.NewPasswordEntry()
	unlock := widget.NewButton("Unlock backup", func() {
		secret := entry.Text
		entry.SetText("")
		reader, err := fynestorage.Reader(uri)
		if err != nil {
			f.unlock(uri, err.Error())
			return
		}
		f.decode(reader, secret)
	})
	unlock.Importance = widget.HighImportance
	unlock.Disable()
	entry.OnChanged = func(value string) {
		if value == "" {
			unlock.Disable()
		} else {
			unlock.Enable()
		}
	}
	entry.OnSubmitted = func(string) {
		if !unlock.Disabled() {
			unlock.OnTapped()
		}
	}
	feedback := newErrorText(message)
	if message == "" {
		feedback.Hide()
	}
	f.page("Unlock backup", func() { entry.SetText(""); f.selectFile("") }, container.NewVBox(newText("Backup password", captionStyle), entry, feedback, unlock))
}

func (f *restoreFlow) review(backup *storage.PawBackup, collision bool, message string) {
	name := widget.NewEntry()
	name.SetText(backup.Name)
	password := NewPasswordField()
	confirm := widget.NewPasswordEntry()
	summary := container.NewVBox(newText("Original vault: "+backup.Name, bodyStyle), newText(fmt.Sprintf("%d collections · %d credentials", backup.Collections, backup.Credentials), bodyStyle))
	if backup.CreationAt != nil {
		summary.Add(newText("Created: "+time.Unix(int64(*backup.CreationAt), 0).UTC().Format("2006-01-02 15:04 UTC"), captionStyle))
	}
	if backup.ExportedAt != 0 {
		summary.Add(newText("CXF document date: "+time.Unix(int64(backup.ExportedAt), 0).UTC().Format("2006-01-02 15:04 UTC"), captionStyle))
	}
	if collision {
		summary.Add(newText("This vault already exists. An independent copy will be created.", captionStyle))
	}
	form := widget.NewForm(widget.NewFormItem("New vault name", name), widget.NewFormItem("New master password", password), widget.NewFormItem("Confirm master password", confirm))
	form.Orientation = widget.Vertical
	create := widget.NewButton("Create vault", func() {
		newName, secret := strings.TrimSpace(name.Text), password.Entry.Text
		password.Entry.SetText("")
		confirm.SetText("")
		ctx, cancel := context.WithCancel(context.Background())
		// Stay on this page until cancellation completes, so a committed restore is reported.
		f.busy("Restoring vault", cancel)
		go func() {
			vault, err := storage.RestorePawBackup(ctx, f.store, backup, newName, secret)
			cancel()
			fyne.Do(func() {
				if errors.Is(err, context.Canceled) {
					f.review(backup, collision, "")
					return
				}
				if err != nil {
					f.review(backup, collision, restoreErrorMessage(err))
					return
				}
				f.complete(vault, vault.Accounts[0].ID != backup.AccountID)
			})
		}()
	})
	create.Importance = widget.HighImportance
	create.Disable()
	update := func() {
		if strings.TrimSpace(name.Text) != "" && password.Entry.Text != "" && password.Entry.Text == confirm.Text {
			create.Enable()
		} else {
			create.Disable()
		}
	}
	wasEmpty := true
	password.OnChanged = func(value string) {
		if (value == "") != wasEmpty {
			form.Refresh()
			wasEmpty = value == ""
		}
		update()
	}
	name.OnChanged = func(string) { update() }
	confirm.OnChanged = func(string) { update() }
	back := func() { password.Entry.SetText(""); confirm.SetText(""); f.selectFile("") }
	feedback := newErrorText(message)
	if message == "" {
		feedback.Hide()
	}
	f.page("Review backup", back, container.NewBorder(nil, container.NewVBox(feedback, create), nil, nil, container.NewVScroll(container.NewVBox(summary, form))))
}

func (f *restoreFlow) complete(vault *model.Vault, copy bool) {
	message := "Vault restored successfully."
	if copy {
		message = "An independent vault copy was restored successfully."
	}
	open := func() { f.restored(vault) }
	button := widget.NewButton("Open vault", open)
	button.Importance = widget.HighImportance
	f.page("Restore complete", open, container.NewVBox(newText(message, bodyStyle), button))
}
