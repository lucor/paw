// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/storage"
)

func TestAttachmentDetailDisplaysEmptyAndMultipleFiles(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)

	empty := newAttachmentDetailSection(window, attachmentDetailActions{})
	window.SetContent(empty)
	require.Contains(t, strings.Join(auditTexts(empty), "\n"), "Attachments · 0")
	require.Contains(t, strings.Join(auditTexts(empty), "\n"), "No attachments")
	require.NotNil(t, detailButton(empty, "Add"))

	files := []cxf.File{testAttachmentFile("same name.txt", 12), testAttachmentFile("same name.txt", 1024)}
	require.NotEqual(t, files[0].ID, files[1].ID)
	view := newAttachmentDetailSection(window, attachmentDetailActions{files: files})
	window.SetContent(view)
	texts := strings.Join(auditTexts(view), "\n")
	require.Contains(t, texts, "Attachments · 2")
	require.Contains(t, texts, "12 B")
	require.Contains(t, texts, "1.0 KiB")
	require.NotNil(t, detailIconButton(view, "Save same name.txt"))
	require.NotNil(t, detailIconButton(view, "Remove same name.txt"))
}

func TestAttachmentDetailRowFitsNarrowWindow(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(320, 640))
	filename := strings.Repeat("long-", 20) + ".txt"
	view := newAttachmentDetailSection(window, attachmentDetailActions{
		files: []cxf.File{testAttachmentFile(filename, 1024)},
	})
	window.SetContent(view)

	var name, size *widget.Label
	for _, object := range test.LaidOutObjects(view) {
		label, ok := object.(*widget.Label)
		if !ok {
			continue
		}
		switch label.Text {
		case filename:
			name = label
		case "1.0 KiB":
			size = label
		}
	}
	require.NotNil(t, name)
	require.NotNil(t, size)
	require.Equal(t, fyne.TextTruncateEllipsis, name.Truncation)
	require.Less(t, name.Position().Y, size.Position().Y)
}

func TestAttachmentDetailActionsAndRemoveConfirmation(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	file := testAttachmentFile("report.pdf", 4)
	var added bool
	var saved, removed string
	view := newAttachmentDetailSection(window, attachmentDetailActions{
		files: []cxf.File{file},
		add:   func() { added = true },
		save:  func(selected cxf.File) { saved = selected.ID },
		remove: func(selected cxf.File) {
			removed = selected.ID
		},
	})
	window.SetContent(view)
	window.Resize(fyne.NewSize(420, 640))
	test.Tap(detailButton(view, "Add"))
	require.True(t, added)
	test.Tap(detailIconButton(view, "Save report.pdf"))
	require.Equal(t, file.ID, saved)
	test.Tap(detailIconButton(view, "Remove report.pdf"))
	require.Contains(t, strings.Join(auditTexts(window.Canvas().Overlays().Top()), "\n"), "Remove attachment?")
	test.Tap(detailButton(window.Canvas().Overlays().Top(), "Cancel"))
	require.Empty(t, removed)
	test.Tap(detailIconButton(view, "Remove report.pdf"))
	test.Tap(detailButton(window.Canvas().Overlays().Top(), "Remove attachment"))
	require.Equal(t, file.ID, removed)
}

func TestAttachmentSaveFilenameRejectsPaths(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	saved := ""
	showAttachmentFilename(window, t.TempDir(), "report.pdf", func(path string) { saved = path })
	form := window.Canvas().Overlays().Top()
	var filename *widget.Entry
	for _, object := range test.LaidOutObjects(form) {
		if entry, ok := object.(*widget.Entry); ok {
			filename = entry
		}
	}
	require.NotNil(t, filename)
	filename.SetText("../outside.pdf")
	test.Tap(detailButton(form, "Save"))
	require.Empty(t, saved)
	require.NotNil(t, window.Canvas().Overlays().Top(), "invalid names must show an error")
}

func TestAttachmentPickersCanBeCancelled(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	selected := false
	chooseAttachmentFile(window, func(fyne.URIReadCloser) { selected = true })
	test.Tap(detailButton(window.Canvas().Overlays().Top(), "Cancel"))
	require.False(t, selected)

	saved := false
	chooseAttachmentDestination(window, testAttachmentFile("report.pdf", 1), func(string) { saved = true })
	test.Tap(detailButton(window.Canvas().Overlays().Top(), "Cancel"))
	require.False(t, saved)
}

func TestAttachmentDetailActionsAreLimitedToNoteAndLogin(t *testing.T) {
	application := &app{}
	for _, kind := range []string{"note", "login"} {
		actions, err := application.attachmentDetailActions(detailItemFixture(t, kind), func() {})
		require.NoError(t, err)
		require.NotNil(t, actions)
	}
	for _, kind := range []string{"password", "ssh"} {
		actions, err := application.attachmentDetailActions(detailItemFixture(t, kind), func() {})
		require.NoError(t, err)
		require.Nil(t, actions)
	}
}

func TestAttachmentRemoveFailureIsReported(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	vault, err := store.CreateVault("Personal", "synthetic-password")
	require.NoError(t, err)
	item := detailFixture(t)
	require.NoError(t, store.StoreItem(vault, item))
	_, err = store.AddAttachment(vault, item.ID, "report.pdf", strings.NewReader("content"))
	require.NoError(t, err)
	item, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	application := &app{win: window, storage: store, vault: vault}
	actions, err := application.attachmentDetailActions(item, func() {})
	require.NoError(t, err)
	require.NoError(t, store.Close())

	view := newAttachmentDetailSection(window, *actions)
	window.SetContent(view)
	test.Tap(detailIconButton(view, "Remove report.pdf"))
	test.Tap(detailButton(window.Canvas().Overlays().Top(), "Remove attachment"))
	message := strings.Join(auditTexts(window.Canvas().Overlays().Top()), "\n")
	require.Contains(t, message, "could not open item")
}

func TestFormatAttachmentSize(t *testing.T) {
	require.Equal(t, "0 B", formatAttachmentSize(0))
	require.Equal(t, "1023 B", formatAttachmentSize(1023))
	require.Equal(t, "1.0 KiB", formatAttachmentSize(1024))
	require.Equal(t, "1.0 MiB", formatAttachmentSize(1<<20))
}

func TestSavingStaleEditorPreservesAttachmentsAddedLater(t *testing.T) {
	a, _, store, item := newHistoryAppFixture(t)
	proposed := *item
	proposed.Title = "Edited title"
	_, err := store.AddAttachment(a.vault, item.ID, "new attachment.txt", strings.NewReader("content"))
	require.NoError(t, err)

	a.saveEditedItem(item, &proposed, false)
	updated, err := store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Edited title", updated.Title)
	files, err := updated.Files()
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "new attachment.txt", files[0].Name)
}

func TestSavingUnchangedEditorKeepsCredentialOrderAndHistory(t *testing.T) {
	a, _, store, item := newHistoryAppFixture(t)
	_, err := store.AddAttachment(a.vault, item.ID, "file.txt", strings.NewReader("content"))
	require.NoError(t, err)
	current, err := store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	current.Credentials = append(current.Credentials, jsontext.Value(`{"type":"note","content":{"fieldType":"string","value":"additional note"}}`))
	require.NoError(t, store.StoreItem(a.vault, current))
	before, err := current.StateHash()
	require.NoError(t, err)
	proposed := *current
	a.saveEditedItem(current, &proposed, false)
	stored, err := store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	after, err := stored.StateHash()
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, current.Credentials, stored.Credentials)
}

func testAttachmentFile(name string, size uint64) cxf.File {
	digest := sha256.Sum256(nil)
	idBytes := make([]byte, 16)
	for index := range idBytes {
		idBytes[index] = byte(size >> (uint(index) * 8))
	}
	return cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            base64.RawURLEncoding.EncodeToString(idBytes),
		Name:          name,
		DecryptedSize: size,
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
}
