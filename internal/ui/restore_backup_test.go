// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

func TestRestoreReviewShowsBackupMetadataAndValidatesNewVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Restore")
	t.Cleanup(window.Close)
	store := &storage.StorageMock{}
	store.OnVaults = func() ([]model.VaultInfo, error) {
		return []model.VaultInfo{{Name: "Personal"}}, nil
	}
	flow := &restoreFlow{window: window, store: store, back: func() {}, restored: func(*model.Vault) {}}
	created := uint64(1705055040)
	exported := uint64(time.Date(2026, time.October, 30, 0, 0, 0, 0, time.UTC).Unix())
	backup := &storage.PawBackup{Name: "Personal", CreationAt: &created, ExportedAt: exported, Collections: 2, Credentials: 42}
	flow.review(backup, "")
	window.Resize(fyne.NewSize(440, 800))
	window.Show()

	texts := auditTexts(window.Content())
	for _, expected := range []string{"Review backup", "Backup contents", "Original vault", "Personal", "Collections", "2", "Items", "42", "Created", "12 Jan 2024", "Exported", "30 Oct 2026", "New vault", "A separate vault will be created. Existing vaults won't be changed.", "A vault with this name already exists. A separate vault will be created."} {
		require.Contains(t, texts, expected)
	}

	var fields []*widget.Entry
	var create *widget.Button
	var form *widget.Form
	for _, object := range test.LaidOutObjects(window.Content()) {
		switch object := object.(type) {
		case *widget.Entry:
			fields = append(fields, object)
		case *widget.Form:
			form = object
		case *widget.Button:
			if object.Text == "Create vault" {
				create = object
			}
		}
	}
	require.Len(t, fields, 3)
	require.NotNil(t, form)
	require.Equal(t, []string{"Vault name", "New master password", "Confirm password"}, []string{form.Items[0].Text, form.Items[1].Text, form.Items[2].Text})
	require.Equal(t, "Personal", fields[0].Text)
	require.NotNil(t, create)
	require.True(t, create.Disabled())

	fields[1].SetText("strong master password")
	fields[2].SetText("different password")
	window.Canvas().Focus(nil)
	require.True(t, create.Disabled())
	require.ErrorContains(t, fields[2].Validate(), "passwords do not match")
	fields[2].SetText("strong master password")
	require.NoError(t, fields[2].Validate())
	require.False(t, create.Disabled(), "a duplicate display name does not block an independent restore")

	fields[0].SetText("Work")
	require.NotContains(t, auditTexts(window.Content()), "A vault with this name already exists. A separate vault will be created.")
	fields[0].SetText(" Personal ")
	require.Contains(t, auditTexts(window.Content()), "A vault with this name already exists. A separate vault will be created.")
	require.False(t, create.Disabled())
}

func TestRestoreReviewOmitsMissingMetadataAndFitsNarrowWindow(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Restore")
	t.Cleanup(window.Close)
	window.Resize(fyne.NewSize(320, 540))
	window.Show()

	flow := &restoreFlow{window: window, store: &storage.StorageMock{VaultStorageMock: storage.VaultStorageMock{
		OnVaults: func() ([]model.VaultInfo, error) { return nil, nil },
	}}, back: func() {}, restored: func(*model.Vault) {}}
	flow.review(&storage.PawBackup{Collections: 0, Credentials: 0}, "")
	texts := auditTexts(window.Content())
	require.Contains(t, texts, "Collections")
	require.Contains(t, texts, "Items")
	require.NotContains(t, texts, "Original vault")
	require.NotContains(t, texts, "Created")
	require.NotContains(t, texts, "Exported")
	require.NotContains(t, texts, "Untitled")

	var scroll *container.Scroll
	for _, object := range test.LaidOutObjects(window.Content()) {
		if candidate, ok := object.(*container.Scroll); ok {
			scroll = candidate
		}
	}
	require.NotNil(t, scroll, "the narrow review form should be vertically scrollable")
	require.Greater(t, scroll.Content.Size().Height, float32(0))
}

func TestRestoreUnlockRequiresBackupPassword(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Restore")
	t.Cleanup(window.Close)
	flow := &restoreFlow{window: window, store: &storage.StorageMock{}, back: func() {}, restored: func(*model.Vault) {}}
	flow.unlock(nil, "")
	window.Show()
	var password *widget.Entry
	var unlock *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		switch object := object.(type) {
		case *widget.Entry:
			password = object
		case *widget.Button:
			if object.Text == "Unlock backup" {
				unlock = object
			}
		}
	}
	require.NotNil(t, password)
	require.True(t, password.Password)
	require.NotNil(t, unlock)
	require.True(t, unlock.Disabled())
	require.Contains(t, auditTexts(window.Content()), "Backup password")
	require.NotContains(t, auditTexts(window.Content()), "Master password")
	password.SetText("backup secret")
	require.False(t, unlock.Disabled())
}

func TestRestoreBackupFilterMatchesOnlySupportedExtensions(t *testing.T) {
	filter := restoreBackupFileFilter{}
	for _, name := range []string{"backup.cxf", "backup.cxf.age", "BACKUP.CXF", "BACKUP.CXF.AGE"} {
		require.True(t, filter.Matches(fynestorage.NewFileURI(name)), name)
	}
	for _, name := range []string{"backup.age", "backup.zip", "backup.cxf.txt", "backup.cxf.age.txt"} {
		require.False(t, filter.Matches(fynestorage.NewFileURI(name)), name)
	}
}

func TestRestoreFilePickerOpensInListView(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Restore")
	t.Cleanup(window.Close)
	window.Resize(fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight))
	flow := &restoreFlow{window: window, store: &storage.StorageMock{}, back: func() {}, restored: func(*model.Vault) {}}
	flow.selectFile("")
	window.Show()
	windowCount := len(app.Driver().AllWindows())
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Select backup file" {
			test.Tap(button)
			break
		}
	}

	require.Len(t, app.Driver().AllWindows(), windowCount, "the picker should remain in the Paw window")
	pickerDialog := window.Canvas().Overlays().Top()
	require.NotNil(t, pickerDialog)
	require.Equal(t, fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight), pickerDialog.Size())
	var fileLists int
	for _, object := range test.LaidOutObjects(pickerDialog) {
		if _, ok := object.(*widget.List); ok {
			fileLists++
		}
	}
	require.GreaterOrEqual(t, fileLists, 2, "the picker should show the file listing as a list alongside its favorites list")
	for _, object := range test.LaidOutObjects(pickerDialog) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Cancel" {
			test.Tap(button)
			break
		}
	}
	require.Nil(t, window.Canvas().Overlays().Top(), "closing the picker should dismiss the modal dialog")
}

func TestRestoreCompleteShowsVaultNameAndOpensRestoredVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Restore")
	t.Cleanup(window.Close)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal copy")
	require.NoError(t, err)
	var opened *model.Vault
	flow := &restoreFlow{window: window, store: &storage.StorageMock{}, back: func() {}, restored: func(restored *model.Vault) { opened = restored }}
	flow.complete(vault)
	window.Show()
	require.Contains(t, auditTexts(window.Content()), "Vault restored")
	require.Contains(t, auditTexts(window.Content()), "Personal copy is ready to use.")
	var open *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Open vault" {
			open = button
		}
	}
	require.NotNil(t, open)
	test.Tap(open)
	require.Same(t, vault, opened)
}
