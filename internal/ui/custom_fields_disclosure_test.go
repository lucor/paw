// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json/jsontext"
	"testing"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestCustomFieldsDisclosureEmptyAndExpandedStates(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	disclosure := newCustomFieldsDisclosure(editor)
	fynetest.NewTempWindow(t, disclosure.View)

	require.True(t, disclosure.expanded)
	require.Len(t, disclosure.View.Objects, 2)
	require.Equal(t, "Custom fields", disclosure.heading.Text)
	addField := findButtonByText(t, editor.View, "Add field")
	addSection := findButtonByText(t, editor.View, "Add section")
	require.NotNil(t, addField)
	require.NotNil(t, addSection)
	require.Equal(t, addField.Position().Y, addSection.Position().Y)
	require.False(t, disclosure.content.Hidden)
	require.Empty(t, editor.groups)
	for _, object := range fynetest.LaidOutObjects(disclosure.header) {
		if button, ok := object.(*iconButton); ok {
			require.NotEqual(t, "Add custom fields", button.AccessibilityLabel())
		}
	}
	fynetest.Tap(disclosure.heading)
	require.True(t, disclosure.content.Hidden)
	fynetest.Tap(disclosure.heading)
	require.False(t, disclosure.content.Hidden)
}

func TestCustomFieldsDisclosureHeaderMenuAddsFieldAndSection(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	disclosure := newCustomFieldsDisclosure(editor)
	window := fynetest.NewTempWindow(t, disclosure.View)

	chooseCustomFieldType(t, window, editor.View, "Text")
	require.True(t, disclosure.populated)
	require.True(t, disclosure.expanded)
	require.Equal(t, "Custom fields · 1 fields", disclosure.heading.Text)

	chooseCustomFieldsAction(t, window, disclosure.add, "Add field")
	confirmCustomField(t, window, "Hidden", "Add")
	require.Equal(t, "Custom fields · 2 fields", disclosure.heading.Text)

	chooseCustomFieldsAction(t, window, disclosure.add, "Add section")
	confirmCustomSection(t, window, "Recovery codes", "Add")
	require.True(t, editor.hasSections())
	require.Equal(t, "Custom fields · 2 fields", disclosure.heading.Text)
}

func TestCustomFieldsDisclosureStaysOpenWhenLastFieldIsRemoved(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	item.Credentials = append(item.Credentials,
		jsontext.Value(`{"type":"custom-fields","fields":[{"fieldType":"string","value":"one"}]}`),
		jsontext.Value(`{"type":"custom-fields","label":"Section","fields":[{"fieldType":"string","value":"two"}]}`),
	)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	disclosure := newCustomFieldsDisclosure(editor)
	window := fynetest.NewTempWindow(t, disclosure.View)

	require.Equal(t, "Custom fields · 2 fields", disclosure.heading.Text)
	require.True(t, disclosure.populated)
	require.False(t, disclosure.content.Hidden)
	chooseCustomFieldsAction(t, window, disclosure.add, "Add field")
	cancelCustomFieldDialog(t, window)
	require.Equal(t, "Custom fields · 2 fields", disclosure.heading.Text)
	chooseCustomFieldsAction(t, window, disclosure.add, "Add section")
	cancelCustomFieldDialog(t, window)
	require.Len(t, editor.groups, 2)

	var fieldActions []*iconButton
	for _, object := range fynetest.LaidOutObjects(editor.View) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Field actions" {
			fieldActions = append(fieldActions, button)
		}
	}
	require.Len(t, fieldActions, 2)
	chooseCustomFieldsAction(t, window, fieldActions[0], "Remove")
	require.Equal(t, "Custom fields · 1 fields", disclosure.heading.Text)
	fieldActions = nil
	for _, object := range fynetest.LaidOutObjects(editor.View) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Field actions" {
			fieldActions = append(fieldActions, button)
		}
	}
	require.Len(t, fieldActions, 1)
	chooseCustomFieldsAction(t, window, fieldActions[0], "Remove")
	require.True(t, disclosure.populated, "an empty named section counts as content")
	require.Equal(t, "Custom fields", disclosure.heading.Text)
	sectionActions := findIconButtonByLabel(t, editor.View, "Section actions")
	chooseCustomFieldsAction(t, window, sectionActions, "Remove section")
	require.False(t, disclosure.populated)
	require.True(t, disclosure.expanded)
	require.Equal(t, "Custom fields", disclosure.heading.Text)
	require.NotNil(t, findButtonByText(t, editor.View, "Add field"))
	require.NotNil(t, findButtonByText(t, editor.View, "Add section"))
}

func cancelCustomFieldDialog(t *testing.T, window fyne.Window) {
	t.Helper()
	var cancel *widget.Button
	for _, object := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Cancel" {
			cancel = button
			break
		}
	}
	require.NotNil(t, cancel)
	fynetest.Tap(cancel)
}
