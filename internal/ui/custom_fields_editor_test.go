package ui

import (
	"encoding/json/jsontext"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestCustomFieldsEditorReplacesFieldWithoutInheritingUnknownData(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, jsontext.Value(`{"type":"custom-fields","fields":[{"fieldType":"string","value":"old","future":{"keep":true}}]}`))
	editor, err := NewNoteEditor(item, false)
	require.NoError(t, err)
	fynetest.NewTempWindow(t, editor.View)
	fields := editor.customFields
	remove := findDeleteAction(fields.View)
	require.NotNil(t, remove)
	fynetest.Tap(remove)
	add := findSelect(fields.View)
	require.NotNil(t, add)
	add.SetSelected("Text")
	fields.groups[0].fields[0].value.SetText("old")
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

func findSelect(object fyne.CanvasObject) *widget.Select {
	if selectWidget, ok := object.(*widget.Select); ok {
		return selectWidget
	}
	if group, ok := object.(*fyne.Container); ok {
		for _, child := range group.Objects {
			if selectWidget := findSelect(child); selectWidget != nil {
				return selectWidget
			}
		}
	}
	return nil
}

func TestCustomFieldsEditorHiddenAndSave(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := NewNoteEditor(item, false)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	_ = window
	groupView := editor.customFields.View.Objects[0].(*fyne.Container)
	selectField := groupView.Objects[len(groupView.Objects)-1].(*widget.Select)
	selectField.SetSelected("Hidden")
	require.Empty(t, selectField.Selected)
	require.Len(t, editor.customFields.groups, 1)
	group := &editor.customFields.groups[0]
	field := &group.fields[0]
	field.label.SetText("Secret")
	field.value.SetText("synthetic")
	require.True(t, field.value.Password)
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Equal(t, cxf.FieldTypeConcealedString, groups[0].Credential.Fields[0].FieldType)
	original, err := item.CustomFields()
	require.NoError(t, err)
	require.Empty(t, original)
}

func TestCustomFieldsEditorAddRemoveGroupsAndFields(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor, err := newCustomFieldsEditor(item)
	require.NoError(t, err)
	window := fynetest.NewTempWindow(t, editor.View)
	window.Resize(fyne.NewSize(320, 700))
	group := editor.View.Objects[0].(*fyne.Container)
	add := group.Objects[len(group.Objects)-1].(*widget.Select)
	add.SetSelected("Text")
	require.Empty(t, add.Selected)
	group = editor.View.Objects[0].(*fyne.Container)
	add = group.Objects[len(group.Objects)-1].(*widget.Select)
	add.SetSelected("Hidden")
	require.Empty(t, add.Selected)
	require.Len(t, editor.groups[0].fields, 2)
	fieldRow := editor.View.Objects[0].(*fyne.Container).Objects[0].(*fyne.Container)
	deleteField := findDeleteAction(fieldRow.Objects[0])
	require.NotNil(t, deleteField)
	require.Greater(t, deleteField.Size().Width, float32(0))
	require.False(t, editor.groups[0].fields[0].value.Password)
	require.True(t, editor.groups[0].fields[1].value.Password)
	fynetest.Tap(deleteField)
	require.Len(t, editor.groups[0].fields, 1)
	fynetest.Tap(editor.View.Objects[len(editor.View.Objects)-1].(*widget.Button))
	require.Len(t, editor.groups, 2)
	sectionView := editor.View.Objects[2].(*fyne.Container)
	deleteSection := findDeleteAction(sectionView.Objects[0])
	require.NotNil(t, deleteSection)
	require.Greater(t, deleteSection.Size().Width, float32(0))
	sectionSelect := findSelect(sectionView)
	require.NotNil(t, sectionSelect)
	sectionSelect.SetSelected("Text")
	sectionView = editor.View.Objects[2].(*fyne.Container)
	deleteSection = findDeleteAction(sectionView.Objects[0])
	require.NotNil(t, deleteSection)
	_, isSeparator := editor.View.Objects[1].(*widget.Separator)
	require.True(t, isSeparator)
	fynetest.Tap(deleteSection)
	require.True(t, editor.groups[1].removed)
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
			addField := group.Objects[len(group.Objects)-1].(*widget.Select)
			fieldType := testCase.fieldType
			if fieldType == "" {
				fieldType = "Text"
			}
			addField.SetSelected(fieldType)
			editor.groups[0].fields[0].label.SetText(testCase.fieldLabel)
			editor.groups[0].fields[0].value.SetText(testCase.value)

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
	fieldRow := editor.View.Objects[0].(*fyne.Container).Objects[0].(*fyne.Container)
	deleteField := findDeleteAction(fieldRow)
	require.NotNil(t, deleteField)
	fynetest.Tap(deleteField)
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
	fynetest.Tap(editor.View.Objects[len(editor.View.Objects)-1].(*widget.Button))
	editor.groups[0].label.SetText("Recovery codes")
	updated := cloneItem(item)
	require.NoError(t, editor.Apply(updated))
	groups, err := updated.CustomFields()
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "Recovery codes", groups[0].Credential.Label)
	require.Empty(t, groups[0].Credential.Fields)
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
			selectField := findSelect(editor.View.Objects[0])
			require.NotNil(t, selectField)
			selectField.SetSelected("Hidden")
			updated := cloneItem(item)
			require.NoError(t, editor.Apply(updated))
			groups, err := updated.CustomFields()
			require.NoError(t, err)
			require.Empty(t, groups)
			require.Equal(t, originalCredentials, updated.Credentials)
		})
	}
}
