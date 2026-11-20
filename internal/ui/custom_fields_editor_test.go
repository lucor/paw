package ui

import (
	"encoding/json/jsontext"
	"slices"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestIsLongTextThresholdAndUnicode(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "newline", value: "first\nsecond", want: true},
		{name: "159 runes", value: strings.Repeat("界", 159)},
		{name: "160 runes", value: strings.Repeat("界", 160)},
		{name: "161 runes", value: strings.Repeat("界", 161), want: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, isLongText(testCase.value))
		})
	}
}

func TestCustomFieldsEditorSupportsFourPresentations(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		multiline    bool
		password     bool
		fieldType    string
		presentation string
	}{
		{name: "Text", fieldType: cxf.FieldTypeString, presentation: "Text"},
		{name: "Hidden", password: true, fieldType: cxf.FieldTypeConcealedString, presentation: "Hidden"},
		{name: "Multiline", multiline: true, fieldType: cxf.FieldTypeString, presentation: "Multiline"},
		{name: "Hidden multiline", multiline: true, password: true, fieldType: cxf.FieldTypeConcealedString, presentation: "Hidden multiline"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fynetest.NewTempApp(t)
			item, err := model.NewNote("Example", "note")
			require.NoError(t, err)
			editor, err := newCustomFieldsEditor(item)
			require.NoError(t, err)
			window := fynetest.NewTempWindow(t, editor.View)
			chooseCustomFieldType(t, window, editor.View, testCase.presentation)
			field := editor.groups[0].fields[0].field
			require.Equal(t, testCase.fieldType, field.FieldType)
			if testCase.multiline {
				require.Equal(t, "first\nsecond", field.Value)
			} else {
				require.Equal(t, "draft", field.Value)
			}
			require.Equal(t, testCase.presentation, customFieldPresentation(field))
			entry := customFieldValueEntry(testCase.presentation)
			require.Equal(t, testCase.multiline, entry.MultiLine)
			require.Equal(t, testCase.password, entry.Password)
			updated := cloneItem(item)
			require.NoError(t, editor.Apply(updated))
			reopened, err := newCustomFieldsEditor(updated)
			require.NoError(t, err)
			require.Equal(t, testCase.presentation, customFieldPresentation(reopened.groups[0].fields[0].field))
		})
	}
}

func TestCustomFieldsEditorLongValuesUsePreviewAndCurrentDraft(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	value := strings.Repeat("draft value ", 18)
	secret := strings.Repeat("hidden value ", 18)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{
		Type: cxf.CredentialTypeCustomFields,
		Fields: []cxf.EditableField{
			{FieldType: cxf.FieldTypeString, Label: "Draft", Value: value},
			{FieldType: cxf.FieldTypeConcealedString, Label: "Hidden", Value: secret},
		},
	}))
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	window.Resize(fyne.NewSize(360, 640))

	preview, truncated := detailTextPreview(value)
	require.True(t, truncated)
	require.Contains(t, auditTexts(editor.View), preserveLineEdgeSpaces(preview))
	require.NotContains(t, auditTexts(editor.View), value)
	require.NotContains(t, auditTexts(editor.View), secret)
	require.NotContains(t, auditTexts(editor.View), "••••••••••••")
	showMore := findButtonByText(t, editor.View, "Show more")
	require.NotNil(t, showMore)
	copy := detailIconButton(editor.View, "Copy Draft")
	require.NotNil(t, copy)
	fynetest.Tap(copy)
	require.Equal(t, value, fyne.CurrentApp().Clipboard().Content())
	fynetest.Tap(showMore)
	require.Contains(t, auditTexts(window.Content()), preserveLineEdgeSpaces(value))
	fynetest.Tap(detailIconButton(window.Content(), "Back"))
	require.Same(t, editor.View, window.Content())

	updatedDraft := "unsaved draft\n" + strings.Repeat("current value ", 14)
	editor.groups[0].fields[0].field.Value = updatedDraft
	editor.refresh()
	fynetest.Tap(findButtonByText(t, editor.View, "Show more"))
	require.Contains(t, auditTexts(window.Content()), preserveLineEdgeSpaces(updatedDraft))
	fynetest.Tap(detailIconButton(window.Content(), "Back"))

	eye := detailIconButton(editor.View, "Show Hidden")
	require.NotNil(t, eye)
	require.Nil(t, detailIconButton(editor.View, "Copy Hidden"))
	fynetest.Tap(eye)
	require.Contains(t, auditTexts(window.Content()), preserveLineEdgeSpaces(secret))
	fynetest.Tap(detailIconButton(window.Content(), "Back"))
}

func TestCustomFieldsEditorDoesNotPreviewUnknownFieldTypes(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	value := "future value\n" + strings.Repeat("additional future text ", 10)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{
		Type: cxf.CredentialTypeCustomFields,
		Fields: []cxf.EditableField{{
			FieldType: "future-type",
			Label:     "Future field",
			Value:     value,
		}},
	}))
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	fynetest.NewTempWindow(t, editor.View)

	require.Contains(t, auditTexts(editor.View), value)
	require.Nil(t, findButtonByText(t, editor.View, "Show more"))
}

func TestCustomFieldDialogKeepsSelectedTypeUntilReopened(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	var updated cxf.EditableField
	var updatedPresentation string
	editor.showFieldDialog(cxf.EditableField{FieldType: cxf.FieldTypeString, Label: "Long"}, "Text", true, func(field cxf.EditableField, presentation string) {
		updated = field
		updatedPresentation = presentation
	})

	var kind *widget.Select
	var entries []*widget.Entry
	var save *widget.Button
	for _, object := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		switch object := object.(type) {
		case *widget.Select:
			kind = object
		case *widget.Entry:
			entries = append(entries, object)
		case *widget.Button:
			if object.Text == "Save" {
				save = object
			}
		}
	}
	require.NotNil(t, kind)
	require.NotNil(t, save)
	require.Len(t, entries, 2)
	require.Equal(t, "Text", kind.Selected)
	longValue := strings.Repeat("é", multilineThreshold+1)
	entries[1].SetText(longValue)
	require.Equal(t, "Text", kind.Selected, "typing a long value does not override the current selection")
	fynetest.Tap(save)
	require.Equal(t, longValue, updated.Value)
	require.Equal(t, "Text", updatedPresentation)

	editor.showFieldDialog(updated, customFieldPresentation(updated), true, func(cxf.EditableField, string) {})
	kind = nil
	entries = nil
	var cancel *widget.Button
	for _, object := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		switch object := object.(type) {
		case *widget.Select:
			kind = object
		case *widget.Entry:
			entries = append(entries, object)
		case *widget.Button:
			if object.Text == "Cancel" {
				cancel = object
			}
		}
	}
	require.NotNil(t, kind)
	require.NotNil(t, cancel)
	require.Equal(t, "Multiline", kind.Selected)
	require.True(t, entries[1].MultiLine)
	fynetest.Tap(cancel)
}

// TestCustomFieldPresentationChoiceDoesNotDirtyTheItem keeps presentation-only changes out of CXF edits.
func TestCustomFieldPresentationChoiceDoesNotDirtyTheItem(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	value := strings.Repeat("é", multilineThreshold+1)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{
		Type:   cxf.CredentialTypeCustomFields,
		Fields: []cxf.EditableField{{FieldType: cxf.FieldTypeString, Label: "Long", Value: value}},
	}))
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	require.Equal(t, "Multiline", editor.groups[0].fields[0].presentation)

	window := fynetest.NewTempWindow(t, editor.View)
	fieldActions := findIconButtonByLabel(t, editor.View, "Field actions")
	chooseCustomFieldsAction(t, window, fieldActions, "Edit")
	var kind *widget.Select
	var save *widget.Button
	for _, object := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		switch object := object.(type) {
		case *widget.Select:
			kind = object
		case *widget.Button:
			if object.Text == "Save" {
				save = object
			}
		}
	}
	require.NotNil(t, kind)
	require.Equal(t, "Multiline", kind.Selected)
	kind.SetSelected("Text")
	require.NotNil(t, save)
	fynetest.Tap(save)
	require.Equal(t, "Text", editor.groups[0].fields[0].presentation)
	require.False(t, editor.IsDirty())
	fieldActions = findIconButtonByLabel(t, editor.View, "Field actions")
	chooseCustomFieldsAction(t, window, fieldActions, "Edit")
	kind = nil
	var cancel *widget.Button
	for _, object := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		switch object := object.(type) {
		case *widget.Select:
			kind = object
		case *widget.Button:
			if object.Text == "Cancel" {
				cancel = object
			}
		}
	}
	require.NotNil(t, kind)
	require.Equal(t, "Text", kind.Selected, "the explicit choice remains in this editor session")
	require.NotNil(t, cancel)
	fynetest.Tap(cancel)

	updated := cloneItem(item)
	require.NoError(t, editor.Apply(updated))
	require.Equal(t, item.Credentials, updated.Credentials)
	reopened, err := newCustomFieldsEditor(updated)
	require.NoError(t, err)
	require.Equal(t, "Multiline", reopened.groups[0].fields[0].presentation)
}

func TestCustomFieldsEditorReplacesFieldWithoutInheritingUnknownData(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, jsontext.Value(`{"type":"custom-fields","fields":[{"fieldType":"string","value":"old","future":{"keep":true}}]}`))
	editor, err := NewNoteEditor(item, false)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	fields := editor.customFields
	fynetest.Tap(editor.customDisclosure.heading)
	remove := findDeleteAction(fields.View)
	require.NotNil(t, remove)
	chooseCustomFieldsAction(t, window, remove, "Remove")
	chooseCustomFieldType(t, window, fields.View, "Text")
	fields.groups[0].fields[0].field.Label = ""
	fields.groups[0].fields[0].field.Value = "old"
	require.True(t, editor.IsDirty())
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	require.JSONEq(t, `{"type":"custom-fields","fields":[{"fieldType":"string","value":"old"}]}`, string(updated.Credentials[1]))
	require.Contains(t, string(item.Credentials[1]), "future")
}

func findDeleteAction(object fyne.CanvasObject) *iconButton {
	if button, ok := object.(*iconButton); ok {
		return button
	}
	if group, ok := object.(*fyne.Container); ok {
		for _, child := range group.Objects {
			if button := findDeleteAction(child); button != nil {
				return button
			}
		}
	}
	return nil
}

// chooseCustomFieldType adds a field through the dialog.
func chooseCustomFieldType(t *testing.T, window fyne.Window, object fyne.CanvasObject, fieldType string) {
	t.Helper()
	addField := findButtonByText(t, object, "Add field")
	require.NotNil(t, addField)
	fynetest.Tap(addField)
	confirmCustomField(t, window, fieldType, "Add")
}

// confirmCustomField fills the open field dialog and confirms it.
func confirmCustomField(t *testing.T, window fyne.Window, fieldType, action string) {
	t.Helper()
	overlay := window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	var confirm *widget.Button
	for _, control := range fynetest.LaidOutObjects(overlay) {
		switch control := control.(type) {
		case *widget.Select:
			control.SetSelected(fieldType)
		case *widget.Button:
			if control.Text == action {
				confirm = control
			}
		}
	}
	for _, control := range fynetest.LaidOutObjects(overlay) {
		if entry, ok := control.(*widget.Entry); ok {
			if entry.MultiLine {
				entry.SetText("first\nsecond")
			} else {
				entry.SetText("draft")
			}
		}
	}
	require.NotNil(t, confirm)
	fynetest.Tap(confirm)
}

// chooseCustomFieldsAction selects a field or section menu item.
func chooseCustomFieldsAction(t *testing.T, window fyne.Window, button *iconButton, action string) {
	t.Helper()
	fynetest.Tap(button)
	labels := menuLabels(t, window)
	var popup *widget.PopUpMenu
	for _, control := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if menu, ok := control.(*widget.PopUpMenu); ok {
			popup = menu
			break
		}
	}
	require.NotNil(t, popup)
	index := slices.Index(labels, action)
	require.NotEqual(t, -1, index)
	option, ok := popup.Items[index].(fyne.Tappable)
	require.True(t, ok)
	fynetest.Tap(option)
}

func TestCustomFieldsEditorHiddenAndSave(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := NewNoteEditor(item, false)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	groupView := editor.customFields.View.Objects[0].(*fyne.Container)
	addField := findButtonByText(t, groupView, "Add field")
	require.NotNil(t, addField)
	require.Equal(t, icons.ListPlus.Name(), addField.Icon.Name())
	chooseCustomFieldType(t, window, groupView, "Hidden")
	require.Len(t, editor.customFields.groups, 1)
	group := &editor.customFields.groups[0]
	field := &group.fields[0]
	field.field.Label = "Secret"
	field.field.Value = "synthetic"
	require.Equal(t, cxf.FieldTypeConcealedString, field.field.FieldType)
	editor.customFields.refresh()
	reveal := findIconButtonByLabel(t, editor.customFields.View, "Show Secret")
	require.NotNil(t, reveal, "concealed values should expose a reveal action")
	require.NotContains(t, auditTexts(editor.customFields.View), "synthetic", "concealed values start masked")
	fynetest.Tap(reveal)
	require.Contains(t, auditTexts(editor.customFields.View), "synthetic", "the reveal action shows the value")
	fynetest.Tap(reveal)
	require.NotContains(t, auditTexts(editor.customFields.View), "synthetic", "the reveal action masks the value again")
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Equal(t, cxf.FieldTypeConcealedString, groups[0].Credential.Fields[0].FieldType)
	original, err := item.CustomFields()
	require.NoError(t, err)
	require.Empty(t, original)
}

// findIconButtonByLabel locates a visible action by its accessible label.
func findIconButtonByLabel(t *testing.T, object fyne.CanvasObject, label string) *iconButton {
	t.Helper()
	for _, child := range fynetest.LaidOutObjects(object) {
		button, ok := child.(*iconButton)
		if ok && button.AccessibilityLabel() == label {
			return button
		}
	}
	t.Fatalf("icon button %q not found", label)
	return nil
}

func TestCustomFieldsEditorMultilineAndEditCancel(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := NewNoteEditor(item, false)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	window.Resize(fyne.NewSize(320, 540))
	chooseCustomFieldType(t, window, editor.customFields.View, "Multiline")
	require.True(t, editor.customDisclosure.expanded)
	require.Contains(t, auditTexts(editor.customFields.View), "first\nsecond")
	menu := findDeleteAction(editor.customFields.View)
	require.NotNil(t, menu)
	chooseCustomFieldsAction(t, window, menu, "Edit")
	var cancel, save *widget.Button
	var entries []*widget.Entry
	for _, object := range fynetest.LaidOutObjects(window.Canvas().Overlays().Top()) {
		switch object := object.(type) {
		case *widget.Button:
			switch object.Text {
			case "Cancel":
				cancel = object
			case "Save":
				save = object
			}
		case *widget.Entry:
			entries = append(entries, object)
		}
	}
	require.NotNil(t, save)
	require.NotNil(t, cancel)
	require.NotEmpty(t, entries)
	require.True(t, slices.ContainsFunc(entries, func(entry *widget.Entry) bool {
		return entry.MultiLine && entry.Text == "first\nsecond"
	}))
	entries[0].SetText("discarded")
	fynetest.Tap(cancel)
	require.Equal(t, "draft", editor.customFields.groups[0].fields[0].field.Label)
	groups, err := item.CustomFields()
	require.NoError(t, err)
	require.Empty(t, groups)
}

func TestCustomFieldsEditorAddRemoveGroupsAndFields(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	disclosure := newCustomFieldsDisclosure(editor)
	window := fynetest.NewTempWindow(t, disclosure.View)
	window.Resize(fyne.NewSize(320, 700))
	group := editor.View.Objects[0].(*fyne.Container)
	addButton := findButtonByText(t, group, "Add field")
	require.NotNil(t, addButton)
	require.Less(t, addButton.Size().Width, window.Canvas().Size().Width)
	chooseCustomFieldType(t, window, group, "Text")
	require.True(t, disclosure.expanded)
	group = editor.View.Objects[0].(*fyne.Container)
	require.Nil(t, findButtonByText(t, group, "Add field"))
	fynetest.Tap(disclosure.heading)
	require.False(t, disclosure.expanded)
	chooseCustomFieldsAction(t, window, disclosure.add, "Add field")
	confirmCustomField(t, window, "Hidden", "Add")
	require.True(t, disclosure.expanded)
	require.Contains(t, auditTexts(editor.View), "••••••••••••")
	_, separated := editor.View.Objects[0].(*fyne.Container).Objects[1].(*widget.Separator)
	require.True(t, separated)
	require.Len(t, editor.groups[0].fields, 2)
	deleteField := findDeleteAction(editor.View.Objects[0])
	require.NotNil(t, deleteField)
	require.Greater(t, deleteField.Size().Width, float32(0))
	require.Equal(t, cxf.FieldTypeString, editor.groups[0].fields[0].field.FieldType)
	require.Equal(t, cxf.FieldTypeConcealedString, editor.groups[0].fields[1].field.FieldType)
	chooseCustomFieldsAction(t, window, deleteField, "Remove")
	require.Len(t, editor.groups[0].fields, 1)
	fynetest.Tap(disclosure.heading)
	chooseCustomFieldsAction(t, window, disclosure.add, "Add section")
	confirmCustomSection(t, window, "Recovery codes", "Add")
	require.True(t, disclosure.expanded)
	require.Len(t, editor.groups, 2)
	sectionView := editor.View.Objects[1].(*fyne.Container)
	require.Len(t, sectionView.Objects, 2)
	background, ok := sectionView.Objects[0].(*canvas.Rectangle)
	require.True(t, ok)
	require.Equal(t, sectionView.Size(), background.Size())
	deleteSection := findDeleteAction(sectionView)
	require.NotNil(t, deleteSection)
	require.Greater(t, deleteSection.Size().Width, float32(0))
	chooseCustomFieldsAction(t, window, deleteSection, "Rename")
	confirmCustomSection(t, window, "Renamed", "Save")
	require.Equal(t, "Renamed", editor.groups[1].credential.Label)
	deleteSection = findDeleteAction(editor.View.Objects[1])
	require.NotNil(t, deleteSection)
	chooseCustomFieldsAction(t, window, deleteSection, "Add field")
	confirmCustomField(t, window, "Hidden", "Add")
	require.Len(t, editor.groups[1].fields, 1)
	require.Len(t, editor.groups[0].fields, 1)
	deleteSection = findDeleteAction(editor.View.Objects[1])
	require.NotNil(t, deleteSection)
	chooseCustomFieldsAction(t, window, deleteSection, "Remove section")
	require.True(t, editor.groups[1].removed)
	editor.groups[0].fields[0].field.Label = ""
	editor.groups[0].fields[0].field.Value = ""
	updated := cloneItem(item)
	require.NoError(t, editor.Apply(updated))
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Empty(t, groups)
}

func TestCustomFieldsEditorDiscardsOnlyNewCompletelyEmptyFields(t *testing.T) {
	for _, testCase := range []struct {
		name       string
		fieldType  string
		fieldLabel string
		value      string
		wantFields int
	}{
		{name: "empty field", wantFields: 0},
		{name: "empty hidden field", fieldType: "Hidden", wantFields: 0},
		{name: "label only", fieldLabel: "Recovery code", wantFields: 1},
		{name: "value only", value: "synthetic", wantFields: 1},
		{name: "label and value", fieldLabel: "Recovery code", value: "synthetic", wantFields: 1},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			fynetest.NewTempApp(t)
			item, err := model.NewNote("Example", "note")
			require.NoError(t, err)
			editor, err := newCustomFieldsEditor(item)
			require.NoError(t, err)
			group := editor.View.Objects[0].(*fyne.Container)
			fieldType := testCase.fieldType
			if fieldType == "" {
				fieldType = "Text"
			}
			window := fynetest.NewTempWindow(t, editor.View)
			chooseCustomFieldType(t, window, group, fieldType)
			editor.groups[0].fields[0].field.Label = testCase.fieldLabel
			editor.groups[0].fields[0].field.Value = testCase.value

			updated := cloneItem(item)
			require.NoError(t, editor.Apply(updated))
			groups, err := updated.CustomFields()
			require.NoError(t, err)
			if testCase.wantFields == 0 {
				require.Empty(t, groups)
				return
			}
			require.Len(t, groups, 1)
			require.Len(t, groups[0].Credential.Fields, testCase.wantFields)
		})
	}
}

func TestCustomFieldsEditorPreservesExistingEmptyFieldAndSection(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	credential := cxf.CustomFields{
		Type:   cxf.CredentialTypeCustomFields,
		Label:  "Recovery codes",
		Fields: []cxf.EditableField{{FieldType: cxf.FieldTypeString}},
	}
	require.NoError(t, item.AppendCustomFields(credential))
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	updated := cloneItem(item)
	require.NoError(t, editor.Apply(updated))
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "Recovery codes", groups[0].Credential.Label)
	require.Len(t, groups[0].Credential.Fields, 1)
}

func TestCustomFieldsEditorRejectsConcurrentCustomFieldChanges(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	group := cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{}}
	require.NoError(t, item.AppendCustomFields(group))
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	group.Label = "Changed elsewhere"
	require.NoError(t, item.ReplaceCustomFields(1, group, nil))
	require.ErrorContains(t, editor.Apply(cloneItem(item)), "changed while editing")
}

func TestCustomFieldsEditorCanRemoveUnsupportedField(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	require.NoError(t, item.AppendCustomFields(cxf.CustomFields{
		Type:   cxf.CredentialTypeCustomFields,
		Fields: []cxf.EditableField{{FieldType: "future-field", Label: "Opaque", Value: "preserved until removed"}},
	}))
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	fieldRow := editor.View.Objects[0].(*fyne.Container).Objects[0].(*fyne.Container)
	deleteField := findDeleteAction(fieldRow)
	require.NotNil(t, deleteField)
	chooseCustomFieldsAction(t, window, deleteField, "Remove")
	updated := cloneItem(item)
	require.NoError(t, editor.Apply(updated))
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Empty(t, groups[0].Credential.Fields)
}

func TestCustomFieldsEditorKeepsNamedEmptySection(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	addCustomSection(t, window, editor.View)
	updated := cloneItem(item)
	require.NoError(t, editor.Apply(updated))
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "Recovery codes", groups[0].Credential.Label)
	require.Empty(t, groups[0].Credential.Fields)
}

// TestCustomFieldsSectionNameUsesFullWidthEntryAndValidation checks the section dialog input and validation.
func TestCustomFieldsSectionNameUsesFullWidthEntryAndValidation(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	editor.showSectionDialog(-1)

	overlay := window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	var name *widget.Entry
	var confirm *widget.Button
	for _, object := range fynetest.LaidOutObjects(overlay) {
		switch object := object.(type) {
		case *widget.Entry:
			name = object
		case *widget.Button:
			if object.Text == "Add" {
				confirm = object
			}
		}
	}
	require.NotNil(t, name)
	require.NotNil(t, confirm)
	require.True(t, confirm.Disabled(), "an empty section name remains invalid")
	name.SetText("  ")
	require.True(t, confirm.Disabled(), "whitespace-only names remain invalid")
	name.SetText("Recovery codes")
	require.False(t, confirm.Disabled())
	fynetest.Tap(confirm)
	require.Len(t, editor.groups, 1)
	require.Equal(t, "Recovery codes", editor.groups[0].credential.Label)

	editor.showSectionDialog(0)
	overlay = window.Canvas().Overlays().Top()
	name = nil
	confirm = nil
	for _, object := range fynetest.LaidOutObjects(overlay) {
		switch object := object.(type) {
		case *widget.Entry:
			name = object
		case *widget.Button:
			if object.Text == "Save" {
				confirm = object
			}
		}
	}
	require.NotNil(t, name)
	require.NotNil(t, confirm)
	require.False(t, confirm.Disabled(), "a valid existing section name can be saved")
	name.SetText("  ")
	require.True(t, confirm.Disabled(), "whitespace-only names remain invalid")
	name.SetText("Recovery codes updated")
	fynetest.Tap(confirm)
	require.Equal(t, "Recovery codes updated", editor.groups[0].credential.Label)
}

func TestCustomFieldsEditorEmptySaveAcrossItemCategories(t *testing.T) {
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	items := map[string]func(*testing.T) *model.Item{
		"login": func(t *testing.T) *model.Item {
			item, err := model.NewLogin("Example", "user", "secret", settings)
			require.NoError(t, err)
			return item
		},
		"password": func(t *testing.T) *model.Item {
			item, err := model.NewPassword("Example", "secret", settings)
			require.NoError(t, err)
			return item
		},
		"note": func(t *testing.T) *model.Item {
			item, err := model.NewNote("Example", "note")
			require.NoError(t, err)
			return item
		},
		"ssh key": func(t *testing.T) *model.Item {
			item, _ := sshDetailFixture(t)
			return item
		},
	}
	for name, newItem := range items {
		t.Run(name, func(t *testing.T) {
			fynetest.NewTempApp(t)
			item := newItem(t)
			originalCredentials := slices.Clone(item.Credentials)
			editor, err := newCustomFieldsEditor(item)
			require.NoError(t, err)
			window := fynetest.NewTempWindow(t, editor.View)
			chooseCustomFieldType(t, window, editor.View.Objects[0], "Hidden")
			editor.groups[0].fields[0].field.Label = ""
			editor.groups[0].fields[0].field.Value = ""
			updated := cloneItem(item)
			require.NoError(t, editor.Apply(updated))
			groups, err := updated.CustomFields()
			require.NoError(t, err)
			require.Empty(t, groups)
			require.Equal(t, originalCredentials, updated.Credentials)
		})
	}
}

// addCustomSection confirms a named section in the dialog.
func addCustomSection(t *testing.T, window fyne.Window, view fyne.CanvasObject) {
	t.Helper()
	button := findButtonByText(t, view, "Add section")
	require.NotNil(t, button)
	fynetest.Tap(button)
	confirmCustomSection(t, window, "Recovery codes", "Add")
}

// confirmCustomSection enters a section name and confirms the dialog.
func confirmCustomSection(t *testing.T, window fyne.Window, name, action string) {
	t.Helper()
	overlay := window.Canvas().Overlays().Top()
	require.NotNil(t, overlay)
	require.NotContains(t, auditTexts(overlay), "Section name")
	var confirm *widget.Button
	for _, control := range fynetest.LaidOutObjects(overlay) {
		switch control := control.(type) {
		case *widget.Entry:
			control.SetText(name)
		case *widget.Button:
			if control.Text == action {
				confirm = control
			}
		}
	}
	require.NotNil(t, confirm)
	fynetest.Tap(confirm)
}
