// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"errors"
	"fmt"
	"image/color"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type restoreFlow struct {
	window   fyne.Window
	store    storage.Storage
	back     func()
	restored func(*model.Vault)
}

const (
	restorePlainBackupSuffix     = ".cxf"
	restoreEncryptedBackupSuffix = ".cxf.age"
)

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
		picker := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
			if err != nil {
				f.selectFile(err.Error())
				return
			}
			if reader == nil {
				return
			}
			f.decode(reader, "")
		}, f.window)
		configureFilePicker(picker)
		picker.SetFilter(restoreBackupFileFilter{})
		picker.Show()
	})
	choose.Importance = widget.HighImportance
	feedback := newErrorText(message)
	if message == "" {
		feedback.Hide()
	}
	content := container.NewVScroll(container.NewVBox(newText("Select a Paw backup file (.cxf or .cxf.age).", bodyStyle), feedback, choose))
	f.page("Restore from backup", f.back, content)
}

// restoreBackupFileFilter matches Paw backup names, including the compound age extension.
type restoreBackupFileFilter struct{}

func (restoreBackupFileFilter) Matches(uri fyne.URI) bool {
	name := strings.ToLower(uri.Name())
	return strings.HasSuffix(name, restorePlainBackupSuffix) || strings.HasSuffix(name, restoreEncryptedBackupSuffix)
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
			f.review(backup, "")
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
	content := container.NewVScroll(container.NewVBox(newText("Backup password", captionStyle), entry, feedback, unlock))
	f.page("Unlock backup", func() { entry.SetText(""); f.selectFile("") }, content)
}

func (f *restoreFlow) review(backup *storage.PawBackup, message string) {
	name := widget.NewEntry()
	name.SetText(backup.Name)
	password := NewPasswordField()
	confirm := widget.NewPasswordEntry()
	confirm.Validator = func(value string) error {
		if value == "" {
			return errors.New("password confirmation is required")
		}
		if value != password.Entry.Text {
			return errors.New("passwords do not match")
		}
		return nil
	}
	nameWarning := newText("A vault with this name already exists. A separate vault will be created.", captionStyle)
	nameWarning.Hide()
	nameInput := container.NewVBox(name, nameWarning)
	form := widget.NewForm(
		widget.NewFormItem("Vault name", nameInput),
		widget.NewFormItem("New master password", password),
		widget.NewFormItem("Confirm password", confirm),
	)
	form.Orientation = widget.Vertical

	var existingVaults []model.VaultInfo
	if f.store != nil {
		vaults, err := f.store.Vaults()
		if err != nil && message == "" {
			message = restoreErrorMessage(err)
		} else {
			existingVaults = vaults
		}
	}

	var updateCreateButton func()
	updateNameWarning := func() {
		show := restoreVaultNameExists(existingVaults, name.Text)
		if show == nameWarning.Visible() {
			return
		}
		if show {
			nameWarning.Show()
		} else {
			nameWarning.Hide()
		}
		form.Refresh()
	}
	name.OnChanged = func(string) {
		updateNameWarning()
		if updateCreateButton != nil {
			updateCreateButton()
		}
	}

	backupContents := make([]fyne.CanvasObject, 0, 5)
	if backup.Name != "" {
		backupContents = append(backupContents, restoreMetadataRow("Original vault", backup.Name))
	}
	backupContents = append(backupContents,
		restoreMetadataRow("Collections", fmt.Sprint(backup.Collections)),
		restoreMetadataRow("Items", fmt.Sprint(backup.Credentials)),
	)
	if backup.CreationAt != nil {
		backupContents = append(backupContents, restoreMetadataRow("Created", restoreDate(*backup.CreationAt)))
	}
	if backup.ExportedAt != 0 {
		backupContents = append(backupContents, restoreMetadataRow("Exported", restoreDate(backup.ExportedAt)))
	}
	backupSection := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameLineSpacing)), backupContents...)
	newVaultMessage := newText("A separate vault will be created. Existing vaults won't be changed.", captionStyle)
	newVaultSection := container.NewVBox(newText("New vault", headingStyle), form, newVaultMessage)
	contents := container.NewVScroll(container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)),
		newText("Backup contents", headingStyle), backupSection,
		widget.NewSeparator(), newVaultSection,
	))

	feedback := newErrorText(message)
	if message == "" {
		feedback.Hide()
	}
	create := widget.NewButton("Create vault", func() {
		if name.Validate() != nil || form.Validate() != nil || strings.TrimSpace(name.Text) == "" || password.Entry.Text == "" || confirm.Text != password.Entry.Text {
			return
		}
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
					f.review(backup, "")
					return
				}
				if err != nil {
					f.review(backup, restoreErrorMessage(err))
					return
				}
				f.complete(vault)
			})
		}()
	})
	create.Importance = widget.HighImportance
	create.Disable()
	updateCreateButton = func() {
		if strings.TrimSpace(name.Text) == "" || password.Entry.Text == "" || confirm.Text != password.Entry.Text {
			create.Disable()
			return
		}
		create.Enable()
	}
	wasEmpty := true
	password.OnChanged = func(value string) {
		if (value == "") != wasEmpty {
			form.Refresh()
			wasEmpty = value == ""
		}
		if confirm.Text != "" && confirm.Text == password.Entry.Text {
			confirm.Validate()
		}
		updateCreateButton()
	}
	confirm.OnChanged = func(string) {
		if confirm.Text == password.Entry.Text {
			confirm.Validate()
		}
		updateCreateButton()
	}
	updateNameWarning()
	updateCreateButton()
	back := func() { password.Entry.SetText(""); confirm.SetText(""); f.selectFile("") }
	f.page("Review backup", back, container.NewBorder(nil, container.NewVBox(feedback, create), nil, nil, contents))
}

func restoreMetadataRow(label, value string) fyne.CanvasObject {
	labelText := newText(label, captionStyle)
	valueLabel := newText(value, bodyStyle)
	valueLabel.Alignment = fyne.TextAlignTrailing
	return container.NewBorder(nil, nil, labelText, valueLabel)
}

func restoreDate(timestamp uint64) string {
	return time.Unix(int64(timestamp), 0).UTC().Format("2 Jan 2006")
}

func restoreVaultNameExists(vaults []model.VaultInfo, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, vault := range vaults {
		if strings.EqualFold(strings.TrimSpace(vault.Name), name) {
			return true
		}
	}
	return false
}

func (f *restoreFlow) complete(vault *model.Vault) {
	open := func() { f.restored(vault) }
	circle := canvas.NewCircle(color.Transparent)
	circle.StrokeColor = theme.Color(theme.ColorNameForeground)
	circle.StrokeWidth = 1
	check := container.NewGridWrap(fyne.NewSquareSize(16), widget.NewIcon(icons.Check))
	status := container.NewHBox(container.NewGridWrap(fyne.NewSquareSize(32), container.NewStack(circle, container.NewCenter(check))), newText("Vault restored", headingStyle))
	openButton := widget.NewButton("Open vault", open)
	openButton.Importance = widget.HighImportance
	content := container.NewVScroll(container.NewVBox(status, newText(vault.Metadata.VaultName+" is ready to use.", bodyStyle), openButton))
	f.page("Restore complete", open, content)
}
