// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

func TestExportCredentialsViewDefaultsAndValidation(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	vault := testExportVault(t, "Personal")
	view := newExportCredentialsView(window, nil, vault, "Personal", func() {})
	window.Resize(fyne.NewSize(400, 640))
	window.SetContent(view.View)

	require.Equal(t, exportEncryptedLabel, view.Mode.Selected)
	require.True(t, view.Password.Entry.Password)
	require.True(t, view.Confirm.Password)
	require.True(t, view.ExportButton.Disabled())
	require.Contains(t, auditTexts(window.Content()), "Export credentials")
	require.Nil(t, window.Canvas().Overlays().Top(), "the form is a view, not a modal")

	view.Password.Entry.SetText("export password")
	require.True(t, view.ExportButton.Disabled())
	view.Confirm.SetText("wrong")
	view.Confirm.FocusLost()
	require.ErrorContains(t, view.Confirm.Validate(), "passwords do not match")
	require.True(t, view.confirmError.Visible())
	require.Equal(t, "Passwords do not match.", view.confirmError.Text)
	require.True(t, view.ExportButton.Disabled())
	view.Confirm.SetText("export password")
	require.NoError(t, view.Confirm.Validate(), "matching confirmation clears the error immediately")
	require.False(t, view.confirmError.Visible())
	require.False(t, view.ExportButton.Disabled())

	view.Mode.SetSelected(exportPlaintextLabel)
	require.False(t, view.passwordFields.Visible())
	require.True(t, view.warning.Visible())
	require.True(t, view.Consent.Visible())
	require.True(t, view.ExportButton.Disabled())
	test.Tap(view.Consent)
	require.False(t, view.ExportButton.Disabled())
	view.Mode.SetSelected(exportEncryptedLabel)
	view.Mode.SetSelected(exportPlaintextLabel)
	require.False(t, view.Consent.Checked, "plaintext consent resets when entering that mode")
	require.True(t, view.ExportButton.Disabled())
}

func TestExportCredentialsSavePickerUsesVaultNameAndCancelKeepsForm(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	vault := testExportVault(t, "Personal")
	view := newExportCredentialsView(window, nil, vault, "Personal", func() {})
	window.Resize(fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight))
	window.SetContent(view.View)
	view.Password.Entry.SetText("export password")
	view.Confirm.SetText("export password")
	test.Tap(view.ExportButton)

	picker := window.Canvas().Overlays().Top()
	require.NotNil(t, picker)
	require.Equal(t, fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight), picker.Size())
	var filename *widget.Entry
	var cancel *widget.Button
	var fileLists int
	for _, object := range test.LaidOutObjects(picker) {
		switch object := object.(type) {
		case *widget.List:
			fileLists++
		case *widget.Entry:
			if object.Text == "Personal.cxf.age" {
				filename = object
			}
		case *widget.Button:
			if object.Text == "Cancel" {
				cancel = object
			}
		}
	}
	require.NotNil(t, filename)
	require.NotNil(t, cancel)
	require.GreaterOrEqual(t, fileLists, 2, "the picker should use list view for favorites and files")
	filename.SetText("renamed.cxf.age")
	test.Tap(cancel)

	require.Nil(t, window.Canvas().Overlays().Top())
	require.Same(t, view.View, window.Content())
	require.Equal(t, "export password", view.Password.Entry.Text)
	require.Equal(t, "export password", view.Confirm.Text)
}

func TestExportCredentialsRejectsAttachmentsBeforeOpeningPicker(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	vault, err := store.CreateVault("Personal", "synthetic-password")
	require.NoError(t, err)
	item, err := model.NewNote("Note", "content")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	_, err = store.AddAttachment(vault, item.ID, "file.txt", strings.NewReader("payload"))
	require.NoError(t, err)

	view := newExportCredentialsView(window, store, vault, "Personal", func() {})
	view.Password.Entry.SetText("export password")
	view.Confirm.SetText("export password")
	test.Tap(view.ExportButton)
	require.Nil(t, window.Canvas().Overlays().Top(), "unsupported export must be rejected before opening a writer dialog")
	require.Contains(t, view.errorText.Text, "cannot include attachment files")
}

func TestPawExportOptionAcceptsAttachmentsAndOpensFolderPicker(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	vault, err := store.CreateVault("Personal", "synthetic-password")
	require.NoError(t, err)
	item, err := model.NewNote("Note", "content")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	_, err = store.AddAttachment(vault, item.ID, "file.txt", strings.NewReader("payload"))
	require.NoError(t, err)

	view := newExportCredentialsView(window, store, vault, "Personal", func() {})
	window.SetContent(view.View)
	view.Mode.SetSelected(exportPawLabel)
	view.Password.Entry.SetText("archive password")
	view.Confirm.SetText("archive password")
	var disabled bool
	fyne.DoAndWait(func() { disabled = view.ExportButton.Disabled() })
	require.False(t, disabled)
	test.Tap(view.ExportButton)
	var overlay fyne.CanvasObject
	var message string
	fyne.DoAndWait(func() {
		overlay = window.Canvas().Overlays().Top()
		message = view.errorText.Text
	})
	require.NotNil(t, overlay, "Paw archive export must ask for a folder without creating a file-save writer")
	require.Empty(t, message, "the Paw format preserves attachments")
}

func TestExportCredentialsBackClearsPasswords(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	vault := testExportVault(t, "Work")
	backCalls := 0
	view := newExportCredentialsView(window, nil, vault, "Work", func() { backCalls++ })
	view.Password.Entry.SetText("temporary")
	view.Confirm.SetText("temporary")

	view.leave()

	require.Equal(t, 1, backCalls)
	require.Empty(t, view.Password.Entry.Text)
	require.Empty(t, view.Confirm.Text)
}

func TestExportFilenameSanitizesUnsafeNames(t *testing.T) {
	require.Equal(t, "vault.cxf.age", exportFilename("../", false))
	require.Equal(t, "vault.cxf", exportFilename("CON", true))
	require.Equal(t, "Work- Paw-.cxf.age", exportFilename(`Work: Paw?`, false))
	require.Equal(t, "vault.paw", pawExportFilename("../"))
	require.Equal(t, "Personal.paw", pawExportFilename("Personal.paw"))
}

func TestExportVaultActionOpensSecondaryView(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	vault := testExportVault(t, "Personal")
	a := &app{win: window, vault: vault}
	a.exportToFile()

	require.Contains(t, auditTexts(window.Content()), "Export credentials")
	for _, object := range test.LaidOutObjects(window.Content()) {
		_, isNavigation := object.(*Navigation)
		require.False(t, isNavigation, "secondary export view does not show bottom navigation")
	}
}

func TestExportFilenameUsesVaultNameAndSelectedFormat(t *testing.T) {
	tests := []struct {
		name      string
		plaintext bool
		want      string
	}{
		{name: "Personal", want: "Personal.cxf.age"},
		{name: "Personal.cxf", want: "Personal.cxf.age"},
		{name: "Personal.cxf.age", plaintext: true, want: "Personal.cxf"},
		{name: `Work/Projects: Paw?`, want: "Work-Projects- Paw-.cxf.age"},
		{name: `../`, want: "vault.cxf.age"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, exportFilename(tt.name, tt.plaintext))
		})
	}
}

func TestExportFilenameUsesActiveVault(t *testing.T) {
	vault := testExportVault(t, "Personal")
	require.Equal(t, "Personal.cxf.age", exportFilename(vaultName(vault), false))
	require.Equal(t, "Personal.cxf", exportFilename(vaultName(vault), true))
	require.Equal(t, "Personal.paw", pawExportFilename(vaultName(vault)))
}

func testExportVault(t *testing.T, name string) *model.Vault {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, name)
	require.NoError(t, err)
	return vault
}
