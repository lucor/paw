// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"path/filepath"
	"strings"
)

// ShowExportCredentials offers encrypted export by default and explicit plaintext consent.
func ShowExportCredentials(window fyne.Window, store storage.Storage, vault *model.Vault) {
	mode := widget.NewRadioGroup([]string{"Encrypted (recommended)", "Plaintext"}, nil)
	password := NewPasswordField()
	confirm := widget.NewPasswordEntry()
	name := widget.NewEntry()
	name.SetText("credentials.cxf.age")
	passwords := container.NewVBox(newText("Export password", captionStyle), password, newText("Confirm export password", captionStyle), confirm)
	warning := newText(storage.PlaintextExportWarning, captionStyle)
	warning.Hide()
	consent := widget.NewCheck("I understand and want to export in plaintext", nil)
	consent.Hide()
	mode.OnChanged = func(value string) {
		if value == "Plaintext" {
			passwords.Hide()
			warning.Show()
			consent.Show()
			name.SetText("credentials.cxf")
		} else {
			passwords.Show()
			warning.Hide()
			consent.Hide()
			consent.SetChecked(false)
			name.SetText("credentials.cxf.age")
		}
	}
	mode.SetSelected("Encrypted (recommended)")
	message := newErrorText("")
	content := container.NewVBox(mode, passwords, warning, consent, newText("File name", captionStyle), name, message)
	var options dialog.Dialog
	export := widget.NewButton("Choose destination", func() {
		plaintext := mode.Selected == "Plaintext"
		secret := password.Entry.Text
		if plaintext {
			secret = ""
		}
		filename := strings.TrimSpace(name.Text)
		extension := ".cxf.age"
		if plaintext {
			extension = ".cxf"
		}
		if filename == "" || filename == "." || filepath.Base(filename) != filename || strings.ContainsAny(filename, `/\`) {
			message.SetText("Enter a file name without a directory.")
			return
		}
		if !strings.HasSuffix(filename, extension) {
			filename += extension
		}
		if plaintext && !consent.Checked {
			message.SetText("Confirm that you understand the plaintext warning.")
			return
		}
		if !plaintext && (secret == "" || secret != confirm.Text) {
			message.SetText("Enter an export password and matching confirmation.")
			return
		}
		picker := dialog.NewFolderOpen(func(folder fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, window)
				return
			}
			if folder == nil {
				return
			}
			if folder.Scheme() != "file" {
				dialog.ShowError(fmt.Errorf("export requires a local destination folder"), window)
				return
			}
			options.Hide()
			password.Entry.SetText("")
			confirm.SetText("")
			ctx, cancel := context.WithCancel(context.Background())
			progress := dialog.NewCustom("Exporting credentials", "Cancel", widget.NewProgressBarInfinite(), window)
			progress.SetOnClosed(cancel)
			progress.Show()
			go func() {
				err := storage.ExportFile(ctx, store, vault, filepath.Join(folder.Path(), filename), secret, plaintext)
				cancel()
				fyne.Do(func() {
					progress.Hide()
					if err != nil {
						dialog.ShowError(err, window)
					} else {
						dialog.ShowInformation("Export credentials", "Credentials exported successfully.", window)
					}
				})
			}()
		}, window)
		picker.Show()
	})
	export.Importance = widget.HighImportance
	content.Add(export)
	options = dialog.NewCustom("Export credentials", "Cancel", content, window)
	options.SetOnClosed(func() { password.Entry.SetText(""); confirm.SetText("") })
	options.Resize(fyne.NewSize(440, 460))
	options.Show()
}
