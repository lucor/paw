// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	json "encoding/json/v2"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func noteEditorFixture(t *testing.T) *model.Item {
	t.Helper()
	item, err := model.NewNote("Recovery codes", "First private line.\nSecond private line.")
	require.NoError(t, err)
	totp, err := json.Marshal(cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 30, Digits: 6}, json.Deterministic(true))
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, totp)
	return item
}

func TestNoteEditorControlsAndInteractions(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	editor, err := NewNoteEditor(noteEditorFixture(t), false)
	require.NoError(t, err)
	window.SetContent(editor.View)
	window.Resize(fyne.NewSize(360, 640))
	require.Equal(t, "Edit note", editor.headerTitle.Text)
	require.Nil(t, editor.Title.Validator)
	require.True(t, editor.Notes.MultiLine)
	rowsHeight := editor.Notes.MinSize().Height
	comparison := widget.NewMultiLineEntry()
	comparison.SetMinRowsVisible(6)
	require.Equal(t, comparison.MinSize().Height, rowsHeight, "note content should start with six visible rows")
	editor.Notes.SetText(strings.Repeat("long note line\n", 40))
	require.Equal(t, rowsHeight, editor.Notes.MinSize().Height, "long notes should scroll inside the entry without growing it")
	objects := test.LaidOutObjects(editor.View)
	var labels []string
	for _, object := range objects {
		if label, ok := object.(*widget.Label); ok && (label.Text == "Title" || label.Text == "Notes") {
			labels = append(labels, label.Text)
		}
		_, isNavigation := object.(*Navigation)
		require.False(t, isNavigation)
	}
	require.ElementsMatch(t, []string{"Title", "Notes"}, labels)
	require.NotContains(t, auditTexts(editor.View), "Tags")
	addField := findButtonByText(t, editor.customFields.View, "Add field")
	require.NotNil(t, addField)
	require.Equal(t, icons.ListPlus.Name(), addField.Icon.Name())
	var body *container.Scroll
	for _, object := range test.LaidOutObjects(editor.View) {
		if scroll, ok := object.(*container.Scroll); ok {
			body = scroll
			break
		}
	}
	require.NotNil(t, body)
	shortHeight := body.Size().Height
	test.Type(editor.Notes, " keyboard input")
	require.Contains(t, editor.Notes.Text, "keyboard input")
	require.Greater(t, editor.SaveButton.Size().Width, float32(0), "Save should remain accessible on a narrow screen")
	position := editor.SaveButton.Position()
	require.LessOrEqual(t, position.X+editor.SaveButton.Size().Width, window.Canvas().Size().Width,
		"Save should remain within the narrow viewport")
	window.Resize(fyne.NewSize(430, 926))
	require.Greater(t, body.Size().Height, shortHeight,
		"the notes page body should grow with the available window height")
}

func TestNoteEditorTabOrderStartsWithTitle(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	editor, err := NewNoteEditor(noteEditorFixture(t), true)
	require.NoError(t, err)
	window.SetContent(editor.View)
	window.Canvas().FocusNext()
	require.Equal(t, editor.Title, window.Canvas().Focused(), "the initial Tab focus should land on Title")
	window.Canvas().FocusNext()
	require.Equal(t, editor.Notes, window.Canvas().Focused())
	window.Canvas().Unfocus()
	window.Canvas().Focus(editor.Title)
	window.Canvas().FocusNext()
	require.Equal(t, editor.Notes, window.Canvas().Focused())
}

func TestNoteEditorUpdatesContentAndPreservesOtherCredentials(t *testing.T) {
	test.NewTempApp(t)
	item := noteEditorFixture(t)
	before, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	editor, err := NewNoteEditor(item, false)
	require.NoError(t, err)
	editor.Title.SetText("Updated recovery codes")
	editor.Notes.SetText("One private line.\nAnother private line.")
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	require.Equal(t, item.ID, updated.ID)
	require.Equal(t, "Updated recovery codes", updated.Title)
	decoded, err := model.DecodeCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "One private line.\nAnother private line.", decoded.Note.Content.Value)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.TOTP.Secret)
	after, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}

func TestNoteEditorValidatesTitleAndTracksDirtyState(t *testing.T) {
	test.NewTempApp(t)
	editor, err := NewNoteEditor(noteEditorFixture(t), true)
	require.NoError(t, err)
	require.Equal(t, "New note", editor.headerTitle.Text)
	require.Empty(t, editor.Title.Text)
	require.True(t, editor.SaveButton.Disabled())
	require.False(t, editor.IsDirty())
	editor.Title.SetText(" ")
	_, err = editor.UpdatedItem()
	require.EqualError(t, err, "title is required")
	require.True(t, editor.HasValidationErrors())
	editor.Title.SetText("Private note")
	require.False(t, editor.SaveButton.Disabled())
	require.False(t, editor.HasValidationErrors())
	require.True(t, editor.IsDirty())
	_, err = editor.UpdatedItem()
	require.NoError(t, err)
}

func TestNoteEditorGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := test.NewTempWindow(t, nil)
			editor, err := NewNoteEditor(noteEditorFixture(t), false)
			require.NoError(t, err)
			window.SetContent(editor.View)
			window.Resize(fyne.NewSize(430, 926))
			test.LaidOutObjects(editor.View)
			require.True(t, editor.SaveButton.Visible())
			require.Greater(t, editor.SaveButton.Size().Width, float32(0))
			test.AssertImageMatches(t, "note-editor-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}
