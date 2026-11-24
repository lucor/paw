// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetest "fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
)

func TestHiddenMultilineEntryEditingAndReveal(t *testing.T) {
	fynetest.NewTempApp(t)
	window := fynetest.NewTempWindow(t, nil)
	entry := newHiddenMultilineEntry("first\n\n界é\n")
	window.SetContent(entry)

	require.Equal(t, "first\n\n界é\n", entry.editor.Text)
	require.Equal(t, "*****\n\n**\n", entry.Text)
	var rendered string
	for _, object := range fynetest.LaidOutObjects(entry) {
		if text, ok := object.(*canvas.Text); ok {
			rendered += text.Text
		}
	}
	require.Contains(t, rendered, "*****")
	require.NotContains(t, rendered, "first")
	entry.TypedShortcut(&fyne.ShortcutSelectAll{})
	require.Equal(t, "first\n\n界é\n", entry.SelectedText())
	cursor := entry.CursorTextOffset()
	entry.toggle()
	require.True(t, entry.shown)
	require.Equal(t, "first\n\n界é\n", entry.Text)
	require.Equal(t, cursor, entry.CursorTextOffset())
	require.Equal(t, "first\n\n界é\n", entry.SelectedText())
	entry.toggle()
	require.Equal(t, "*****\n\n**\n", entry.Text)
	require.Equal(t, "first\n\n界é\n", entry.editor.Text)

	entry.TypedShortcut(&fyne.ShortcutSelectAll{})
	clipboard := fyne.CurrentApp().Clipboard()
	clipboard.SetContent("pasted\ntext\n")
	entry.TypedShortcut(&fyne.ShortcutPaste{Clipboard: clipboard})
	require.Equal(t, "pasted\ntext\n", entry.editor.Text)
	require.Equal(t, "******\n****\n", entry.Text)
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyBackspace})
	require.Equal(t, "pasted\ntext", entry.editor.Text)
	require.Equal(t, "******\n****", entry.Text)

	entry.TypedRune('界')
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	require.Equal(t, maskCustomFieldValue(entry.editor.Text), entry.Text)
	entry.editor.SetText("")
	entry.updateDisplay()
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	require.Equal(t, "\n", entry.editor.Text)
	require.Equal(t, "\n", entry.Text)
}
