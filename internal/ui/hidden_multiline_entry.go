// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"unicode/utf8"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/driver/mobile"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// hiddenMultilineEntry uses a detached Entry for editing and undo history.
// The embedded Entry displays the value or its mask and handles pointer input.
type hiddenMultilineEntry struct {
	widget.Entry
	editor *widget.Entry
	reveal *iconButton
	shown  bool
	shift  *fyne.KeyEvent
}

func newHiddenMultilineEntry(value string) *hiddenMultilineEntry {
	e := &hiddenMultilineEntry{editor: widget.NewMultiLineEntry()}
	e.MultiLine = true
	e.Wrapping = e.editor.Wrapping
	e.ExtendBaseWidget(e)
	// Fyne calls the button handler for pointer and keyboard activation.
	e.reveal = newIconButton("Show value", icons.Show, e.toggle)
	e.ActionItem = e.reveal
	e.editor.SetText(value)
	e.updateDisplay()
	return e
}

// TypedRune inserts the actual rune into the detached editor.
func (e *hiddenMultilineEntry) TypedRune(r rune) {
	if !e.Disabled() {
		e.prepareEdit()
		e.editor.TypedRune(r)
		e.finishEdit()
	}
}

// SelectedText returns the selected original text.
func (e *hiddenMultilineEntry) SelectedText() string {
	return e.editor.SelectedText()
}

// TypedKey applies navigation and editing to the original value.
func (e *hiddenMultilineEntry) TypedKey(key *fyne.KeyEvent) {
	if !e.Disabled() {
		e.prepareEdit()
		e.editor.TypedKey(key)
		e.finishEdit()
	}
}

// TypedShortcut delegates clipboard and editing shortcuts to the editor.
func (e *hiddenMultilineEntry) TypedShortcut(shortcut fyne.Shortcut) {
	if e.Disabled() {
		return
	}
	switch shortcut.(type) {
	case *fyne.ShortcutCopy, *fyne.ShortcutCut:
		if !e.shown {
			return
		}
	}
	e.prepareEdit()
	e.editor.TypedShortcut(shortcut)
	e.finishEdit()
}

// Undo and Redo also handle calls from the Entry context menu.
func (e *hiddenMultilineEntry) Undo() { e.TypedShortcut(&fyne.ShortcutUndo{}) }
func (e *hiddenMultilineEntry) Redo() { e.TypedShortcut(&fyne.ShortcutRedo{}) }

// KeyDown remembers Shift for native selection in the detached editor.
func (e *hiddenMultilineEntry) KeyDown(key *fyne.KeyEvent) {
	e.Entry.KeyDown(key)
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shift = key
	}
}

// KeyUp releases Shift for both pointer and keyboard selection.
func (e *hiddenMultilineEntry) KeyUp(key *fyne.KeyEvent) {
	e.Entry.KeyUp(key)
	if key.Name == desktop.KeyShiftLeft || key.Name == desktop.KeyShiftRight {
		e.shift = nil
	}
}

// FocusLost clears any modifier held when focus leaves the entry.
func (e *hiddenMultilineEntry) FocusLost() {
	e.Entry.FocusLost()
	e.shift = nil
}

// MouseDown mirrors pointer selection from the visible entry to the editor.
func (e *hiddenMultilineEntry) MouseDown(event *desktop.MouseEvent) {
	e.Entry.MouseDown(event)
	e.syncEditorSelection()
}

// Dragged mirrors pointer selection as it changes.
func (e *hiddenMultilineEntry) Dragged(event *fyne.DragEvent) {
	e.Entry.Dragged(event)
	e.syncEditorSelection()
}

// MouseUp applies the final pointer selection to the editor.
func (e *hiddenMultilineEntry) MouseUp(event *desktop.MouseEvent) {
	e.Entry.MouseUp(event)
	e.syncEditorSelection()
}

// TouchUp mirrors mobile cursor and selection changes to the editor.
func (e *hiddenMultilineEntry) TouchUp(event *mobile.TouchEvent) {
	e.Entry.TouchUp(event)
	e.syncEditorSelection()
}

// DoubleTapped mirrors word selection to the editor.
func (e *hiddenMultilineEntry) DoubleTapped(event *fyne.PointEvent) {
	e.Entry.DoubleTapped(event)
	e.syncEditorSelection()
}

func (e *hiddenMultilineEntry) prepareEdit() {
	if e.shift != nil {
		e.Entry.KeyUp(e.shift)
	}
	cursor, anchor := multilineSelection(&e.Entry)
	setMultilineSelection(e.editor, cursor, anchor)
	if e.shift != nil {
		e.editor.KeyDown(e.shift)
	}
}

func (e *hiddenMultilineEntry) finishEdit() {
	if e.shift != nil {
		e.editor.KeyUp(e.shift)
	}
	e.updateDisplay()
	if e.shift != nil {
		e.Entry.KeyDown(e.shift)
	}
}

func (e *hiddenMultilineEntry) syncEditorSelection() {
	cursor, anchor := multilineSelection(&e.Entry)
	setMultilineSelection(e.editor, cursor, anchor)
}

func (e *hiddenMultilineEntry) toggle() {
	e.prepareEdit()
	e.shown = !e.shown
	if e.shown {
		e.reveal.label = "Hide value"
		e.reveal.SetIcon(icons.Hide)
	} else {
		e.reveal.label = "Show value"
		e.reveal.SetIcon(icons.Show)
	}
	e.finishEdit()
	if c := fyne.CurrentApp().Driver().CanvasForObject(e); c != nil {
		c.Focus(e)
	}
}

func (e *hiddenMultilineEntry) updateDisplay() {
	cursor, anchor := multilineSelection(e.editor)
	display := e.editor.Text
	if !e.shown {
		display = maskCustomFieldValue(display)
	}
	if e.Text != display {
		e.Entry.SetText(display)
	}
	setMultilineSelection(&e.Entry, cursor, anchor)
}

func maskCustomFieldValue(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' {
			return r
		}
		return '*'
	}, value)
}

// multilineSelection reads both endpoints using Entry's selection navigation.
func multilineSelection(entry *widget.Entry) (cursor, anchor int) {
	cursor = entry.CursorTextOffset()
	count := utf8.RuneCountInString(entry.SelectedText())
	if count == 0 {
		return cursor, cursor
	}

	// Moving left collapses a selection to its start even when both ends
	// border identical text. Restore the original direction immediately.
	entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyLeft})
	anchor = entry.CursorTextOffset()
	if anchor == cursor {
		anchor += count
	}
	setMultilineSelection(entry, cursor, anchor)
	return cursor, anchor
}

// setMultilineSelection uses logical lines, shared by both unwrapped entries.
func setMultilineSelection(entry *widget.Entry, cursor, anchor int) {
	entry.ClearSelection()
	move := func(offset int) {
		prefix := string([]rune(entry.Text)[:offset])
		entry.CursorRow = strings.Count(prefix, "\n")
		entry.CursorColumn = utf8.RuneCountInString(prefix[strings.LastIndexByte(prefix, '\n')+1:])
	}
	move(anchor)
	if cursor == anchor {
		entry.Refresh()
		return
	}
	shift := &fyne.KeyEvent{Name: desktop.KeyShiftLeft}
	entry.KeyDown(shift)
	key := fyne.KeyRight
	if cursor < anchor {
		key = fyne.KeyLeft
		move(cursor + 1)
	} else {
		move(cursor - 1)
	}
	entry.TypedKey(&fyne.KeyEvent{Name: key})
	entry.KeyUp(shift)
}
