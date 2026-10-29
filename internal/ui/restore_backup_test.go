// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"bytes"
	"context"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"testing"
)

func TestRestoreReviewShowsMetadataAndValidatesNewPassword(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, widget.NewLabel("Vaults"))
	window.Resize(fyne.NewSize(480, 650))
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	var data bytes.Buffer
	require.NoError(t, storage.WriteCXF(context.Background(), &data, nil, vault))
	backup, err := storage.ReadPawBackup(context.Background(), &data, "")
	require.NoError(t, err)
	f := &restoreFlow{window: window, restored: func(*model.Vault) { t.Fatal("invalid form must not restore") }}
	f.review(backup, true, "")
	require.Nil(t, window.Canvas().Overlays().Top(), "restore must use a view")
	view := window.Content()
	require.Contains(t, auditTexts(view), "Original vault: Personal")
	require.Contains(t, auditTexts(view), "0 collections · 0 credentials")
	require.Contains(t, auditTexts(view), "This vault already exists. An independent copy will be created.")
	var entries []*widget.Entry
	var create *widget.Button
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok {
			entries = append(entries, entry)
		}
		if button, ok := object.(*widget.Button); ok && button.Text == "Create vault" {
			create = button
		}
	}
	require.Len(t, entries, 3)
	require.NotNil(t, create)
	require.Equal(t, "Personal", entries[0].Text)
	require.True(t, create.Disabled())
	require.True(t, entries[1].Password)
	require.True(t, entries[2].Password)
	entries[1].SetText("new-master")
	entries[2].SetText("different")
	require.True(t, create.Disabled())
	entries[2].SetText("new-master")
	require.False(t, create.Disabled())
	entries[0].SetText(" ")
	require.True(t, create.Disabled())
	entries[0].SetText("Renamed")
	require.False(t, create.Disabled())
	entries[1].SetText("")
	require.True(t, create.Disabled())
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	require.Empty(t, entries[2].Text, "Back must clear password confirmation")
	require.Contains(t, auditTexts(window.Content()), "Restore from backup")
	require.Nil(t, window.Canvas().Overlays().Top())
}

func TestRestoreSelectionAndUnlockUseViewsWithBack(t *testing.T) {
	test.NewTempApp(t)
	previous := widget.NewLabel("Vaults")
	window := test.NewTempWindow(t, previous)
	window.Resize(fyne.NewSize(440, 650))
	ShowRestoreBackup(window, nil, func(*model.Vault) {})
	require.Nil(t, window.Canvas().Overlays().Top())
	require.Contains(t, auditTexts(window.Content()), "Restore from backup")
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	require.Same(t, previous, window.Content())
	f := &restoreFlow{window: window, back: func() { window.SetContent(previous) }}
	f.unlock(nil, "")
	require.Nil(t, window.Canvas().Overlays().Top())
	var entry *widget.Entry
	var unlock *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		if value, ok := object.(*widget.Entry); ok {
			entry = value
		}
		if button, ok := object.(*widget.Button); ok && button.Text == "Unlock backup" {
			unlock = button
		}
	}
	require.NotNil(t, entry)
	require.True(t, entry.Password)
	require.NotNil(t, unlock)
	require.True(t, unlock.Disabled())
	entry.SetText("backup-password")
	require.False(t, unlock.Disabled())
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	require.Empty(t, entry.Text)
	require.Contains(t, auditTexts(window.Content()), "Restore from backup")
}
