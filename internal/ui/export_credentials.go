// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const (
	exportEncryptedLabel = "Encrypted (recommended)"
	exportPlaintextLabel = "Plaintext"
	exportPawLabel       = "Paw archive (includes attachments)"
	exportPasswordError  = "Enter an export password and matching confirmation."
	exportConsentError   = "Confirm that you understand the plaintext warning."
)

type exportCredentialsView struct {
	window fyne.Window
	store  *storage.FileStorage
	vault  *model.Vault
	back   func()

	View         fyne.CanvasObject
	Mode         *widget.RadioGroup
	Password     *PasswordField
	Confirm      *exportConfirmEntry
	Consent      *widget.Check
	ExportButton *widget.Button
	BackButton   *widget.Button

	passwordFields *fyne.Container
	confirmError   *widget.Label
	warning        *widget.Label
	errorText      *widget.Label
	busy           bool
	confirmTouched bool
}

type exportConfirmEntry struct {
	*widget.Entry
	onFocusLost func()
}

func (e *exportConfirmEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onFocusLost != nil {
		e.onFocusLost()
	}
}

// NewExportCredentialsView builds the dedicated export form for the active vault.
func NewExportCredentialsView(window fyne.Window, store *storage.FileStorage, vault *model.Vault, back func()) fyne.CanvasObject {
	return newExportCredentialsView(window, store, vault, vaultName(vault), back).View
}

func newExportCredentialsView(window fyne.Window, store *storage.FileStorage, vault *model.Vault, vaultName string, back func()) *exportCredentialsView {
	v := &exportCredentialsView{
		window:   window,
		store:    store,
		vault:    vault,
		back:     back,
		Mode:     widget.NewRadioGroup([]string{exportEncryptedLabel, exportPlaintextLabel, exportPawLabel}, nil),
		Password: NewPasswordField(),
		Confirm:  &exportConfirmEntry{Entry: widget.NewPasswordEntry()},
	}
	v.Confirm.SetPlaceHolder("")
	v.Confirm.Validator = func(value string) error {
		if value != "" && value != v.Password.Entry.Text {
			return errors.New("passwords do not match")
		}
		return nil
	}
	v.Password.OnChanged = func(string) {
		if v.confirmTouched {
			v.updateConfirmationError()
		}
		v.updateExportButton()
	}
	v.confirmError = newErrorText("Passwords do not match.")
	v.confirmError.Hide()
	v.Confirm.onFocusLost = func() {
		v.confirmTouched = true
		v.updateConfirmationError()
	}
	v.Confirm.OnChanged = func(string) {
		v.updateConfirmationError()
		v.updateExportButton()
	}
	showPassword := newIconButton("Show export password", icons.Show, nil)
	showPassword.OnTapped = func() {
		v.Password.Entry.Password = !v.Password.Entry.Password
		if v.Password.Entry.Password {
			showPassword.Icon = icons.Show
			showPassword.label = "Show export password"
		} else {
			showPassword.Icon = icons.Hide
			showPassword.label = "Hide export password"
		}
		showPassword.Refresh()
		v.Password.Entry.Refresh()
	}
	v.Password.SetTrailingAction(showPassword)
	v.Mode.Horizontal = false
	v.Mode.SetSelected(exportEncryptedLabel)
	v.Mode.OnChanged = v.modeChanged
	v.passwordFields = container.NewVBox(
		newText("Export password", captionStyle),
		v.Password,
		newText("Confirm export password", captionStyle),
		v.confirmField(),
		v.confirmError,
	)
	v.warning = newText(storage.PlaintextExportWarning, bodyStyle)
	v.warning.Hide()
	v.Consent = widget.NewCheck("I understand and want to export in plaintext", nil)
	v.Consent.OnChanged = func(bool) { v.updateExportButton() }
	v.Consent.Hide()
	v.errorText = newErrorText("")
	v.errorText.Hide()
	v.ExportButton = widget.NewButton("Export…", v.chooseDestination)
	v.ExportButton.Importance = widget.HighImportance

	v.BackButton = widget.NewButtonWithIcon("", icons.Back, v.leave)
	v.BackButton.Importance = widget.LowImportance
	header := newPageHeader(v.BackButton, newText("Export credentials", titleStyle), nil)
	fields := container.NewVBox(v.Mode, v.passwordFields, v.warning, v.Consent, v.errorText)
	footer := container.NewVBox(v.ExportButton)
	v.View = newSecondaryPage(header, footer, container.NewVScroll(fields))
	v.updateExportButton()
	return v
}

func (v *exportCredentialsView) updateConfirmationError() {
	if v.Confirm.Text == "" || v.Confirm.Text == v.Password.Entry.Text {
		v.Confirm.SetValidationError(nil)
		v.confirmError.Hide()
		return
	}
	if v.confirmTouched {
		_ = v.Confirm.Validate()
		v.confirmError.Show()
	}
}

func (v *exportCredentialsView) confirmField() fyne.CanvasObject {
	show := false
	button := newIconButton("Show confirmation", icons.Show, nil)
	button.OnTapped = func() {
		show = !show
		v.Confirm.Password = !show
		if show {
			button.Icon = icons.Hide
			button.label = "Hide confirmation"
		} else {
			button.Icon = icons.Show
			button.label = "Show confirmation"
		}
		button.Refresh()
	}
	return container.NewBorder(nil, nil, nil, button, v.Confirm)
}

func (v *exportCredentialsView) modeChanged(mode string) {
	if mode == exportPlaintextLabel {
		v.passwordFields.Hide()
		v.warning.Show()
		v.Consent.SetChecked(false)
		v.Consent.Show()
	} else {
		v.passwordFields.Show()
		v.warning.Hide()
		v.Consent.Hide()
		v.Consent.SetChecked(false)
	}
	v.setError("")
	v.updateExportButton()
	if v.View != nil {
		v.View.Refresh()
	}
}

func (v *exportCredentialsView) updateExportButton() {
	if v.busy {
		v.ExportButton.Disable()
		v.Mode.Disable()
		v.Password.Entry.Disable()
		v.Confirm.Disable()
		v.Consent.Disable()
		v.BackButton.Disable()
		return
	}
	v.Mode.Enable()
	v.Password.Entry.Enable()
	v.Confirm.Enable()
	v.Consent.Enable()
	v.BackButton.Enable()
	if v.Mode.Selected == exportPlaintextLabel {
		if v.Consent.Checked {
			v.ExportButton.Enable()
		} else {
			v.ExportButton.Disable()
		}
		return
	}
	if v.Password.Entry.Text != "" && v.Password.Entry.Text == v.Confirm.Text {
		v.ExportButton.Enable()
	} else {
		v.ExportButton.Disable()
	}
}

func (v *exportCredentialsView) chooseDestination() {
	plaintext := v.Mode.Selected == exportPlaintextLabel
	pawArchive := v.Mode.Selected == exportPawLabel
	password := v.Password.Entry.Text
	if plaintext {
		if !v.Consent.Checked {
			v.setError(exportConsentError)
			return
		}
		password = ""
	} else if password == "" || password != v.Confirm.Text {
		v.setError(exportPasswordError)
		return
	}
	if !pawArchive {
		if err := storage.CheckExportSupported(context.Background(), v.store, v.vault); err != nil {
			v.setError(err.Error())
			return
		}
	}
	v.setError("")
	if pawArchive {
		v.choosePawDestination(password)
		return
	}

	picker := dialog.NewFileSave(func(writer fyne.URIWriteCloser, err error) {
		if err != nil {
			v.setError(err.Error())
			return
		}
		if writer == nil {
			return
		}
		v.exportSelectedFile(writer, password, plaintext)
	}, v.window)
	picker.SetFileName(exportFilename(vaultName(v.vault), plaintext))
	picker.SetFilter(exportFileFilter{plaintext: plaintext})
	configureFilePicker(picker)
	picker.Show()
}

func (v *exportCredentialsView) choosePawDestination(password string) {
	picker := dialog.NewFolderOpen(func(folder fyne.ListableURI, err error) {
		if err != nil {
			v.setError(err.Error())
			return
		}
		if folder == nil {
			return
		}
		if folder.Scheme() != "file" || folder.Path() == "" {
			v.setError("Choose a local destination folder.")
			return
		}
		v.showPawFilename(folder.Path(), password)
	}, v.window)
	configureFilePicker(picker)
	picker.Show()
}

func (v *exportCredentialsView) showPawFilename(directory, password string) {
	filename := widget.NewEntry()
	filename.SetText(pawExportFilename(vaultName(v.vault)))
	content := container.NewVBox(newText("Choose a name for the encrypted Paw archive.", bodyStyle), widget.NewForm(widget.NewFormItem("File name", filename)))
	dialog.NewCustomConfirm("Export Paw archive", "Export", "Cancel", content, func(confirm bool) {
		if !confirm {
			return
		}
		name := strings.TrimSpace(filename.Text)
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\\`) || filepath.Base(name) != name {
			v.setError("Enter a file name without a path.")
			return
		}
		path := filepath.Join(directory, normalizePawFilename(name))
		v.performPawExport(path, password)
	}, v.window).Show()
}

func (v *exportCredentialsView) performPawExport(path, password string) {
	v.busy = true
	v.ExportButton.SetText("Exporting…")
	v.updateExportButton()
	go func() {
		err := storage.ExportPawFile(context.Background(), v.store, v.vault, path, password)
		fyne.Do(func() {
			v.busy = false
			v.ExportButton.SetText("Export…")
			v.updateExportButton()
			if err != nil {
				v.setError(fmt.Sprintf("Export failed: %v", err))
				return
			}
			fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Paw", Content: "Paw archive exported successfully."})
			v.leave()
		})
	}()
}

func (v *exportCredentialsView) exportSelectedFile(writer fyne.URIWriteCloser, password string, plaintext bool) {
	uri := writer.URI()
	if uri == nil {
		_ = writer.Close()
		v.setError("The selected destination is unavailable.")
		return
	}
	filename := normalizeExportFilename(uri.Name(), plaintext)
	if filename != uri.Name() {
		if err := writer.Close(); err != nil {
			v.setError(err.Error())
			return
		}
		if uri.Scheme() != "file" || uri.Path() == "" {
			v.setError("Choose a local file name with the correct export extension.")
			return
		}
		target := fynestorage.NewFileURI(filepath.Join(filepath.Dir(uri.Path()), filename))
		if err := os.Remove(uri.Path()); err != nil && !os.IsNotExist(err) {
			v.setError(fmt.Sprintf("Prepare export destination: %v", err))
			return
		}
		if exists, _ := fynestorage.Exists(target); exists {
			dialog.ShowConfirm("Overwrite?", "Are you sure you want to overwrite the file\n"+filename+"?", func(ok bool) {
				if !ok {
					return
				}
				next, err := fynestorage.Writer(target)
				if err != nil {
					v.setError(err.Error())
					return
				}
				v.performExport(next, password, plaintext)
			}, v.window)
			return
		}
		next, err := fynestorage.Writer(target)
		if err != nil {
			v.setError(err.Error())
			return
		}
		v.performExport(next, password, plaintext)
		return
	}
	v.performExport(writer, password, plaintext)
}

func (v *exportCredentialsView) performExport(writer fyne.URIWriteCloser, password string, plaintext bool) {
	v.busy = true
	v.ExportButton.SetText("Exporting…")
	v.updateExportButton()
	ctx := context.Background()
	uri := writer.URI()
	go func() {
		err := storage.WriteExport(ctx, writer, v.store, v.vault, password, plaintext)
		closeErr := writer.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil && uri != nil && uri.Scheme() == "file" {
			_ = os.Remove(uri.Path())
		}
		fyne.Do(func() {
			v.busy = false
			v.ExportButton.SetText("Export…")
			v.updateExportButton()
			if err != nil {
				v.setError(fmt.Sprintf("Export failed: %v", err))
				return
			}
			fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "Paw", Content: "Credentials exported successfully."})
			v.leave()
		})
	}()
}

func (v *exportCredentialsView) leave() {
	v.Password.Entry.SetText("")
	v.Confirm.SetText("")
	v.confirmTouched = false
	v.confirmError.Hide()
	if v.back != nil {
		v.back()
	}
}

func (v *exportCredentialsView) setError(message string) {
	if message == "" {
		v.errorText.Hide()
		v.errorText.SetText("")
	} else {
		v.errorText.SetText(message)
		v.errorText.Show()
	}
	if v.View != nil {
		v.View.Refresh()
	}
}

func vaultName(vault *model.Vault) string {
	if vault == nil {
		return "vault"
	}
	name, err := vault.Name()
	if err != nil {
		return "vault"
	}
	return name
}

func exportFilename(vaultName string, plaintext bool) string {
	name := sanitizeExportName(vaultName)
	if name == "" {
		name = "vault"
	}
	return normalizeExportFilename(name, plaintext)
}

func pawExportFilename(vaultName string) string {
	name := sanitizeExportName(vaultName)
	if name == "" {
		name = "vault"
	}
	return normalizePawFilename(name)
}

func normalizePawFilename(name string) string {
	name = sanitizeExportName(filepath.Base(name))
	if strings.EqualFold(name, ".") || strings.EqualFold(name, "..") {
		name = ""
	}
	if strings.HasSuffix(strings.ToLower(name), ".paw") {
		name = name[:len(name)-len(".paw")]
	}
	if name == "" {
		name = "vault"
	}
	return name + ".paw"
}

func sanitizeExportName(name string) string {
	name = strings.Trim(name, "./\\")
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return '-'
		}
		return r
	}, strings.TrimSpace(name))
	name = strings.TrimRight(strings.TrimSpace(name), ". ")
	base := strings.ToUpper(strings.SplitN(name, ".", 2)[0])
	reserved := base == "CON" || base == "PRN" || base == "AUX" || base == "NUL"
	if len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
		reserved = true
	}
	if name == "-" || reserved {
		return ""
	}
	return name
}

func normalizeExportFilename(name string, plaintext bool) string {
	name = sanitizeExportName(filepath.Base(name))
	if strings.EqualFold(name, ".") || strings.EqualFold(name, "..") {
		name = ""
	}
	for _, suffix := range []string{".cxf.age", ".cxf"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			name = name[:len(name)-len(suffix)]
			break
		}
	}
	if name == "" {
		name = "vault"
	}
	if plaintext {
		return name + ".cxf"
	}
	return name + ".cxf.age"
}

type exportFileFilter struct{ plaintext bool }

func (f exportFileFilter) Matches(uri fyne.URI) bool {
	name := strings.ToLower(uri.Name())
	if f.plaintext {
		return strings.HasSuffix(name, ".cxf") && !strings.HasSuffix(name, ".cxf.age")
	}
	return strings.HasSuffix(name, ".cxf.age")
}
