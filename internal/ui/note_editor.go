// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// NoteEditor provides the native form for creating or editing a Note item.
type NoteEditor struct {
	View       fyne.CanvasObject
	BackButton *iconButton
	SaveButton *widget.Button
	Title      *widget.Entry
	Notes      *widget.Entry

	item         *model.Item
	initialTitle string
	initialNotes string
	note         *cxf.Note
	noteIndex    int
	titleError   *widget.Label
	headerTitle  *widget.Label
}

// NewNoteEditor builds the create or edit screen for a Note item.
func NewNoteEditor(item *model.Item, isNew bool) (*NoteEditor, error) {
	if err := item.Validate(); err != nil {
		return nil, err
	}
	if item.Metadata.Type != model.NoteItemType {
		return nil, errors.New("item is not a note")
	}
	decoded, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return nil, err
	}
	if decoded.Note == nil {
		return nil, errors.New("note credential is missing")
	}

	editor := &NoteEditor{
		item:       item,
		note:       decoded.Note,
		noteIndex:  decoded.NoteIndex,
		Title:      widget.NewEntry(),
		Notes:      widget.NewMultiLineEntry(),
		titleError: newErrorText(""),
	}
	if !isNew {
		editor.Title.SetText(item.Title)
	}
	editor.Notes.SetText(decoded.Note.Content.Value)
	editor.initialTitle = editor.Title.Text
	editor.initialNotes = decoded.Note.Content.Value
	editor.titleError.Hide()
	editor.Title.OnChanged = func(value string) {
		updateTitleSaveButton(editor.SaveButton, value)
		if editor.titleError.Visible() && strings.TrimSpace(value) != "" {
			editor.titleError.Hide()
		}
	}
	editor.BackButton = newIconButton("Back", icons.Back, nil)
	editor.BackButton.Importance = widget.LowImportance
	editor.SaveButton = widget.NewButton("Save", nil)
	editor.SaveButton.Importance = widget.HighImportance
	updateTitleSaveButton(editor.SaveButton, editor.Title.Text)
	title := "Edit note"
	if isNew {
		title = "New note"
	}
	editor.headerTitle = newText(title, titleStyle)
	header := newPageHeader(editor.BackButton, editor.headerTitle, editor.SaveButton)
	titleField := container.NewVBox(newText("Title", captionStyle), editor.Title, editor.titleError)
	titleField = container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNamePadding), 0), titleField)
	notesLabel := container.New(layout.NewCustomPaddedLayout(0, 0, theme.Size(theme.SizeNamePadding), 0), newText("Notes", captionStyle))
	notesField := container.NewBorder(notesLabel, nil, nil, nil, editor.Notes)
	body := container.New(layout.NewBorderLayout(titleField, nil, nil, nil), titleField, notesField)
	editor.View = container.New(layout.NewCustomPaddedLayout(16, 24, 24, 24), container.NewBorder(header, nil, nil, nil, body))
	return editor, nil
}

// UpdatedItem validates the form and returns an updated copy of its Note item.
func (e *NoteEditor) UpdatedItem() (*model.Item, error) {
	if strings.TrimSpace(e.Title.Text) == "" {
		err := errors.New("title is required")
		setFieldError(e.titleError, err)
		return nil, err
	}
	setFieldError(e.titleError, nil)
	updated := cloneItem(e.item)
	updated.Title = e.Title.Text
	note := *e.note
	note.Content.FieldType = cxf.FieldTypeString
	note.Content.Value = e.Notes.Text
	if err := updated.SetNote(e.noteIndex, note); err != nil {
		return nil, err
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	return updated, nil
}

// IsDirty reports whether the user changed the title or note content.
func (e *NoteEditor) IsDirty() bool {
	return e.Title.Text != e.initialTitle || e.Notes.Text != e.initialNotes
}

// HasValidationErrors reports whether the editor displays a field error.
func (e *NoteEditor) HasValidationErrors() bool { return e.titleError.Visible() }
