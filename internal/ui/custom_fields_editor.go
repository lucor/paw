// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"bytes"
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// customFieldsEditor edits the custom-fields credentials of an item.
type customFieldsEditor struct {
	View    *fyne.Container
	groups  []customFieldsGroupEditor
	initial []model.CustomFieldsGroup
	raw     [][]byte
}

// customFieldsGroupEditor holds the editable state for one custom-fields group.
type customFieldsGroupEditor struct {
	credential cxf.CustomFields
	index      int
	label      *widget.Entry
	fields     []customFieldEditor
	removed    bool
	section    bool
}

// customFieldEditor holds the editable state for one custom field.
type customFieldEditor struct {
	field         cxf.EditableField
	label         *widget.Entry
	value         *widget.Entry
	originalIndex int
}

// newCustomFieldsEditor builds an editor from an item's custom-fields groups.
func newCustomFieldsEditor(item *model.Item) (*customFieldsEditor, error) {
	groups, err := item.CustomFields()
	if err != nil {
		return nil, err
	}
	e := &customFieldsEditor{View: container.NewVBox(), initial: groups}
	for _, group := range groups {
		e.raw = append(e.raw, append([]byte(nil), item.Credentials[group.Index]...))
		e.groups = append(e.groups, e.groupEditor(group.Credential, group.Index))
	}
	e.refresh()
	return e, nil
}

// groupEditor builds the controls for one custom-fields group.
func (e *customFieldsEditor) groupEditor(value cxf.CustomFields, index int) customFieldsGroupEditor {
	g := customFieldsGroupEditor{credential: value, index: index, label: widget.NewEntry()}
	g.section = value.Label != ""
	g.label.SetPlaceHolder("Section name")
	g.label.SetText(value.Label)
	for index, field := range value.Fields {
		g.fields = append(g.fields, e.fieldEditor(field, index))
	}
	return g
}

// fieldEditor builds the controls for one editable field.
func (e *customFieldsEditor) fieldEditor(value cxf.EditableField, originalIndex int) customFieldEditor {
	entry := widget.NewEntry()
	if value.FieldType == cxf.FieldTypeConcealedString {
		entry = widget.NewPasswordEntry()
	}
	f := customFieldEditor{field: value, label: widget.NewEntry(), value: entry, originalIndex: originalIndex}
	f.label.SetPlaceHolder("Field name")
	f.value.SetPlaceHolder("Value")
	f.label.SetText(value.Label)
	f.value.SetText(value.Value)
	return f
}

// refresh rebuilds the editor layout from its current group state.
func (e *customFieldsEditor) refresh() {
	var objects []fyne.CanvasObject
	var defaultRows []fyne.CanvasObject
	for index := range e.groups {
		g := &e.groups[index]
		if g.removed || g.section {
			continue
		}
		defaultRows = append(defaultRows, e.fieldRows(index, g)...)
	}
	defaultRows = append(defaultRows, e.addFieldSelect(func(field customFieldEditor) {
		for index := range e.groups {
			g := &e.groups[index]
			if !g.section && !g.removed {
				g.fields = append(g.fields, field)
				return
			}
		}
		group := e.groupEditor(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{}}, -1)
		group.fields = append(group.fields, field)
		e.groups = append(e.groups, group)
	}))
	objects = append(objects, container.NewVBox(defaultRows...))
	for index := range e.groups {
		g := &e.groups[index]
		if g.removed || !g.section {
			continue
		}
		objects = append(objects, widget.NewSeparator())
		groupIndex := index
		deleteSection := newIconButton("Delete section", icons.Delete, func() {
			e.groups[groupIndex].removed = true
			e.refresh()
		})
		rows := []fyne.CanvasObject{container.NewBorder(nil, nil, nil, deleteSection, g.label)}
		rows = append(rows, e.fieldRows(index, g)...)
		rows = append(rows, e.addFieldSelect(func(field customFieldEditor) {
			e.groups[groupIndex].fields = append(e.groups[groupIndex].fields, field)
		}))
		objects = append(objects, container.NewVBox(rows...))
	}
	objects = append(objects, widget.NewButtonWithIcon("Add section", icons.Plus, func() {
		group := e.groupEditor(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields, Fields: []cxf.EditableField{}}, -1)
		group.section = true
		e.groups = append(e.groups, group)
		e.refresh()
	}))
	e.View.Objects = objects
	e.View.Refresh()
}

// fieldRows builds the controls for a group's fields.
func (e *customFieldsEditor) fieldRows(groupIndex int, g *customFieldsGroupEditor) []fyne.CanvasObject {
	var rows []fyne.CanvasObject
	for fieldIndex := range g.fields {
		f := &g.fields[fieldIndex]
		if f.field.FieldType != cxf.FieldTypeString && f.field.FieldType != cxf.FieldTypeConcealedString {
			deleteField := newIconButton("Delete field", icons.Delete, func() {
				group := &e.groups[groupIndex]
				group.fields = append(group.fields[:fieldIndex], group.fields[fieldIndex+1:]...)
				e.refresh()
			})
			rows = append(rows, container.NewBorder(nil, nil, nil, deleteField, newDetailField(f.field.Label, newText(f.field.Value, bodyStyle))))
			continue
		}
		currentFieldIndex := fieldIndex
		deleteField := newIconButton("Delete field", icons.Delete, func() {
			group := &e.groups[groupIndex]
			group.fields = append(group.fields[:currentFieldIndex], group.fields[currentFieldIndex+1:]...)
			e.refresh()
		})
		fieldSpacer := canvas.NewRectangle(color.Transparent)
		fieldSpacer.SetMinSize(fyne.NewSize(24, 0))
		valueSpacer := canvas.NewRectangle(color.Transparent)
		valueSpacer.SetMinSize(fyne.NewSize(24, 0))
		rows = append(rows, container.NewVBox(
			container.NewBorder(nil, nil, fieldSpacer, deleteField, f.label),
			container.NewBorder(nil, nil, valueSpacer, nil, f.value),
		))
		if fieldIndex < len(g.fields)-1 {
			rows = append(rows, container.NewPadded(widget.NewSeparator()))
		}
	}
	return rows
}

// addFieldSelect creates a selector that appends a text or hidden field.
func (e *customFieldsEditor) addFieldSelect(add func(customFieldEditor)) *widget.Select {
	selectField := widget.NewSelect([]string{"Text", "Hidden"}, nil)
	selectField.PlaceHolder = "+ Add field"
	selectField.OnChanged = func(selected string) {
		fieldType := cxf.FieldTypeString
		switch selected {
		case "Text":
		case "Hidden":
			fieldType = cxf.FieldTypeConcealedString
		default:
			return
		}
		selectField.ClearSelected()
		field := e.fieldEditor(cxf.EditableField{FieldType: fieldType}, -1)
		add(field)
		e.refresh()
	}
	return selectField
}

// Apply saves the edited groups while preserving unrelated item credentials.
func (e *customFieldsEditor) Apply(item *model.Item) error {
	currentGroups, err := item.CustomFields()
	if err != nil {
		return err
	}
	if len(currentGroups) != len(e.initial) {
		return fmt.Errorf("custom fields changed while editing")
	}
	for index, group := range currentGroups {
		if !bytes.Equal(item.Credentials[group.Index], e.raw[index]) {
			return fmt.Errorf("custom fields changed while editing")
		}
	}
	// Remove groups from the end so earlier credential indices stay valid.
	for i := len(currentGroups) - 1; i >= 0; i-- {
		currentIndex := currentGroups[i].Index
		current := &e.groups[i]
		if current.removed {
			if err := item.RemoveCustomFields(currentIndex); err != nil {
				return err
			}
			continue
		}
		updated := current.credential
		updated.Label = current.label.Text
		fields := make([]cxf.EditableField, 0, len(current.fields))
		originalFields := make([]int, 0, len(current.fields))
		for _, f := range current.fields {
			field := f.field
			if field.FieldType == cxf.FieldTypeString || field.FieldType == cxf.FieldTypeConcealedString {
				field.Label, field.Value = f.label.Text, f.value.Text
			}
			if f.originalIndex == -1 && field.Label == "" && field.Value == "" {
				continue
			}
			fields = append(fields, field)
			originalFields = append(originalFields, f.originalIndex)
		}
		updated.Fields = fields
		if err := item.ReplaceCustomFields(currentIndex, updated, originalFields); err != nil {
			return err
		}
	}
	for _, group := range e.groups {
		if group.index < 0 && !group.removed {
			credential := group.credential
			credential.Label = group.label.Text
			credential.Fields = []cxf.EditableField{}
			for _, f := range group.fields {
				field := f.field
				field.Label, field.Value = f.label.Text, f.value.Text
				if field.Label == "" && field.Value == "" {
					continue
				}
				credential.Fields = append(credential.Fields, field)
			}
			if credential.Label == "" && len(credential.Fields) == 0 {
				continue
			}
			if err := item.AppendCustomFields(credential); err != nil {
				return err
			}
		}
	}
	if err := item.Validate(); err != nil {
		return fmt.Errorf("invalid custom fields: %w", err)
	}
	return nil
}

// IsDirty reports whether the editor contains unsaved custom-field changes.
func (e *customFieldsEditor) IsDirty() bool {
	if len(e.initial) != len(e.groups) {
		return true
	}
	for index, original := range e.initial {
		group := &e.groups[index]
		if group.removed || group.label.Text != original.Credential.Label || len(group.fields) != len(original.Credential.Fields) {
			return true
		}
		for fieldIndex, field := range group.fields {
			before := original.Credential.Fields[fieldIndex]
			if field.originalIndex != fieldIndex || field.label.Text != before.Label || field.value.Text != before.Value {
				return true
			}
		}
	}
	return false
}
