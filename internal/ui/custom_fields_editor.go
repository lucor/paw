// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// customFieldsEditor edits the custom-fields credentials of an item.
type customFieldsEditor struct {
	View     *fyne.Container
	groups   []customFieldsGroupEditor
	initial  []model.CustomFieldsGroup
	raw      [][]byte
	onChange func(int)
	onAdd    func()
}

// customFieldsGroupEditor holds the editable state for one custom-fields group.
type customFieldsGroupEditor struct {
	credential cxf.CustomFields
	index      int
	fields     []customFieldEditor
	removed    bool
	section    bool
}

// customFieldEditor holds the editable state for one custom field.
type customFieldEditor struct {
	field         cxf.EditableField
	presentation  string
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
	g := customFieldsGroupEditor{
		credential: value,
		index:      index,
		section:    value.Label != "",
	}
	for index, field := range value.Fields {
		g.fields = append(g.fields, customFieldEditor{
			field: field, presentation: customFieldPresentation(field), originalIndex: index,
		})
	}
	return g
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
		fields := e.fieldRows(index, g)
		if len(defaultRows) > 0 && len(fields) > 0 {
			defaultRows = append(defaultRows, widget.NewSeparator())
		}
		defaultRows = append(defaultRows, fields...)
	}
	empty := len(defaultRows) == 0 && !e.hasSections()
	if empty {
		addField := widget.NewButtonWithIcon("Add field", icons.ListPlus, func() { e.addField(-1) })
		addSection := widget.NewButtonWithIcon("Add section", icons.Form, func() { e.showSectionDialog(-1) })
		addField.Importance = widget.LowImportance
		addSection.Importance = widget.LowImportance
		defaultRows = append(defaultRows, container.NewHBox(addField, addSection))
	}
	objects = append(objects, container.NewVBox(defaultRows...))
	for index := range e.groups {
		g := &e.groups[index]
		if g.removed || !g.section {
			continue
		}
		groupIndex := index
		sectionActions := customFieldsMenu("Section actions",
			fyne.NewMenuItem("Add field", func() { e.addField(groupIndex) }),
			fyne.NewMenuItem("Rename", func() { e.showSectionDialog(groupIndex) }),
			fyne.NewMenuItem("Remove section", func() {
				e.groups[groupIndex].removed = true
				e.refresh()
			}),
		)
		sectionHeader := container.NewStack(
			canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground)),
			newDetailFieldHeader(g.credential.Label, sectionActions),
		)
		rows := []fyne.CanvasObject{sectionHeader}
		fields := e.fieldRows(index, g)
		if len(fields) > 0 {
			indent := canvas.NewRectangle(color.Transparent)
			indent.SetMinSize(fyne.NewSize(12, 0))
			rows = append(rows, container.NewBorder(nil, nil, indent, nil, container.NewVBox(fields...)))
		}
		content := container.NewVBox(rows...)
		objects = append(objects, newCustomFieldsSectionCard(content))
	}
	e.View.Objects = objects
	e.View.Refresh()
	if e.onChange != nil {
		e.onChange(e.fieldCount())
	}
}

// fieldCount returns the number of editable fields across all retained groups.
func (e *customFieldsEditor) fieldCount() int {
	count := 0
	for _, group := range e.groups {
		if !group.removed {
			count += len(group.fields)
		}
	}
	return count
}

// hasSections reports whether the draft includes a named section.
func (e *customFieldsEditor) hasSections() bool {
	for _, group := range e.groups {
		if group.section && !group.removed {
			return true
		}
	}
	return false
}

// fieldRows builds the controls for a group's fields.
func (e *customFieldsEditor) fieldRows(groupIndex int, g *customFieldsGroupEditor) []fyne.CanvasObject {
	var rows []fyne.CanvasObject
	for fieldIndex := range g.fields {
		if fieldIndex > 0 {
			rows = append(rows, widget.NewSeparator())
		}
		f := g.fields[fieldIndex]
		field := f.field
		edit := fyne.NewMenuItem("Edit", func() {
			e.showFieldDialog(field, g.fields[fieldIndex].presentation, true, func(updated cxf.EditableField, presentation string) {
				e.groups[groupIndex].fields[fieldIndex].field = updated
				e.groups[groupIndex].fields[fieldIndex].presentation = presentation
				e.refresh()
			})
		})
		edit.Disabled = field.FieldType != cxf.FieldTypeString && field.FieldType != cxf.FieldTypeConcealedString
		remove := fyne.NewMenuItem("Remove", func() {
			group := &e.groups[groupIndex]
			group.fields = append(group.fields[:fieldIndex], group.fields[fieldIndex+1:]...)
			e.refresh()
		})
		actions := customFieldsMenu("Field actions", edit, remove)
		value := newText(field.Value, bodyStyle)
		value.Wrapping = fyne.TextWrapBreak
		hidden := field.FieldType == cxf.FieldTypeConcealedString
		long := isLongText(field.Value)
		labelActions := []fyne.CanvasObject{actions}
		var valueActions []fyne.CanvasObject
		valueTitle := field.Label
		if valueTitle == "" {
			valueTitle = "Custom field"
		}
		if hidden && long {
			eye := newIconButton("Show "+valueTitle, icons.Show, func() {
				e.showFullValue(valueTitle, field.Value)
			})
			labelActions = append(labelActions, eye)
		} else if hidden && field.Value != "" {
			secret := &detailSecret{value: field.Value, label: field.Label, mask: "••••••••••••", text: value}
			value.SetText(secret.mask)
			secret.button = newIconButton("Show "+field.Label, icons.Show, secret.Toggle)
			valueActions = append(valueActions, secret.button)
		}
		var displayedValue fyne.CanvasObject
		if field.Value != "" {
			switch {
			case hidden && long:
				displayedValue = nil
			case field.FieldType == cxf.FieldTypeString && long:
				displayedValue = newDetailTextPreview(field.Value, func() {
					e.showFullValue(valueTitle, field.Value)
				})
				copyLabel := field.Label
				if copyLabel == "" {
					copyLabel = "value"
				}
				valueActions = append(valueActions, newIconButton("Copy "+copyLabel, icons.Copy, func() {
					detailCopy(copyLabel, field.Value)
				}))
			default:
				displayedValue = value
			}
		}
		rows = append(rows, newDetailFieldWithActions(field.Label, displayedValue, labelActions, valueActions))
	}
	return rows
}

// showFullValue opens the draft's complete custom-field value.
func (e *customFieldsEditor) showFullValue(title, value string) {
	window := e.window()
	if window == nil {
		return
	}
	previous := window.Content()
	var fullValue *detailFullValueView
	fullValue = newDetailFullValueView(title, value, false, func() {
		fullValue.Clear()
		window.SetContent(previous)
	}, nil)
	window.SetContent(fullValue.View)
}

// customFieldsMenu opens the actions for a field or section.
func customFieldsMenu(label string, items ...*fyne.MenuItem) *iconButton {
	button := newIconButton(label, icons.More, nil)
	button.Importance = widget.LowImportance
	button.OnTapped = func() {
		widget.ShowPopUpMenuAtRelativePosition(fyne.NewMenu("", items...),
			fyne.CurrentApp().Driver().CanvasForObject(button), fyne.NewPos(0, button.Size().Height), button)
	}
	return button
}

// showSectionDialog adds or renames a section in the draft.
func (e *customFieldsEditor) showSectionDialog(index int) {
	window := e.window()
	if window == nil {
		return
	}
	name := widget.NewEntry()
	title, action := "Add section", "Add"
	if index >= 0 {
		title, action = "Rename section", "Save"
		name.SetText(e.groups[index].credential.Label)
	}
	name.Validator = func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("section name is required")
		}
		return nil
	}
	d := dialog.NewCustom(title, "", name, window)
	cancel := widget.NewButtonWithIcon("Cancel", theme.CancelIcon(), d.Hide)
	cancel.Importance = widget.LowImportance
	confirm := widget.NewButtonWithIcon(action, theme.ConfirmIcon(), func() {
		if name.Validate() != nil {
			return
		}
		d.Hide()
		if index >= 0 {
			e.groups[index].credential.Label = name.Text
		} else {
			e.groups = append(e.groups, e.groupEditor(cxf.CustomFields{
				Type:  cxf.CredentialTypeCustomFields,
				Label: name.Text,
			}, -1))
			if e.onAdd != nil {
				e.onAdd()
			}
		}
		e.refresh()
	})
	confirm.Importance = widget.HighImportance
	confirm.Disable()
	setConfirmEnabled := func(value string) {
		if strings.TrimSpace(value) == "" {
			confirm.Disable()
			return
		}
		confirm.Enable()
	}
	name.OnChanged = setConfirmEnabled
	setConfirmEnabled(name.Text)
	d.SetButtons([]fyne.CanvasObject{container.NewHBox(layout.NewSpacer(), cancel, confirm, layout.NewSpacer())})
	sizeCompactDialog(d, window)
	d.Show()
}

// addField opens the dialog for a named section or the first unnamed group.
func (e *customFieldsEditor) addField(index int) {
	e.showFieldDialog(cxf.EditableField{FieldType: cxf.FieldTypeString}, "Text", false, func(field cxf.EditableField, presentation string) {
		if index < 0 {
			for i, group := range e.groups {
				if !group.section && !group.removed {
					index = i
					break
				}
			}
			if index < 0 {
				index = len(e.groups)
				e.groups = append(e.groups, e.groupEditor(cxf.CustomFields{Type: cxf.CredentialTypeCustomFields}, -1))
			}
		}
		group := &e.groups[index]
		group.fields = append(group.fields, customFieldEditor{field: field, presentation: presentation, originalIndex: -1})
		if e.onAdd != nil {
			e.onAdd()
		}
		e.refresh()
	})
}

// window returns the window containing the custom fields editor.
func (e *customFieldsEditor) window() fyne.Window {
	driver := fyne.CurrentApp().Driver()
	canvas := driver.CanvasForObject(e.View)
	for _, window := range driver.AllWindows() {
		if window.Canvas() == canvas {
			return window
		}
	}
	return nil
}

// customFieldPresentation infers the editor option from a stored CXF field.
func customFieldPresentation(field cxf.EditableField) string {
	long := isLongText(field.Value)
	switch {
	case field.FieldType == cxf.FieldTypeConcealedString && long:
		return "Hidden multiline"
	case field.FieldType == cxf.FieldTypeConcealedString:
		return "Hidden"
	case long:
		return "Multiline"
	default:
		return "Text"
	}
}

// customFieldValueEntry creates the entry used for one selected field presentation.
func customFieldValueEntry(presentation string) *widget.Entry {
	switch presentation {
	case "Hidden":
		return widget.NewPasswordEntry()
	case "Multiline", "Hidden multiline":
		return widget.NewMultiLineEntry()
	default:
		return widget.NewEntry()
	}
}

// showFieldDialog commits a field to the draft only after confirmation.
func (e *customFieldsEditor) showFieldDialog(field cxf.EditableField, presentation string, editing bool, save func(cxf.EditableField, string)) {
	window := e.window()
	if window == nil {
		return
	}
	name := widget.NewEntry()
	name.SetText(field.Label)
	if presentation == "" {
		presentation = customFieldPresentation(field)
	}
	valueBox := container.NewStack()
	setValueEntry := func(kind, text string) {
		var input fyne.CanvasObject
		if kind == "Hidden multiline" {
			input = newHiddenMultilineEntry(text)
		} else {
			entry := customFieldValueEntry(kind)
			entry.SetText(text)
			input = entry
		}
		valueBox.Objects = []fyne.CanvasObject{input}
		valueBox.Refresh()
	}
	readValue := func() string {
		if entry, ok := valueBox.Objects[0].(*hiddenMultilineEntry); ok {
			return entry.editor.Text
		}
		return valueBox.Objects[0].(*widget.Entry).Text
	}
	setValueEntry(presentation, field.Value)
	kind := widget.NewSelect([]string{"Text", "Hidden", "Multiline", "Hidden multiline"}, func(selected string) {
		if selected == "" {
			return
		}
		setValueEntry(selected, readValue())
	})
	kind.SetSelected(presentation)
	title, action := "Add field", "Add"
	if editing {
		title, action = "Edit field", "Save"
	}
	form := widget.NewForm(
		widget.NewFormItem("Type", kind),
		widget.NewFormItem("Field name", name),
		widget.NewFormItem("Value", valueBox),
	)
	form.Orientation = widget.Vertical
	d := dialog.NewCustomConfirm(title, action, "Cancel", form, func(ok bool) {
		if !ok {
			return
		}
		value := readValue()
		if !editing && name.Text == "" && value == "" {
			return
		}
		field.Label = name.Text
		field.Value = value
		field.FieldType = cxf.FieldTypeString
		if kind.Selected == "Hidden" || kind.Selected == "Hidden multiline" {
			field.FieldType = cxf.FieldTypeConcealedString
		}
		save(field, kind.Selected)
	}, window)
	sizeCompactDialog(d, window)
	d.Show()
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
		fields := make([]cxf.EditableField, 0, len(current.fields))
		originalFields := make([]int, 0, len(current.fields))
		for _, f := range current.fields {
			field := f.field
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
			credential.Fields = []cxf.EditableField{}
			for _, f := range group.fields {
				field := f.field
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
		if group.removed || group.credential.Label != original.Credential.Label || len(group.fields) != len(original.Credential.Fields) {
			return true
		}
		for fieldIndex, field := range group.fields {
			before := original.Credential.Fields[fieldIndex]
			if field.originalIndex != fieldIndex || field.field.FieldType != before.FieldType || field.field.Label != before.Label || field.field.Value != before.Value {
				return true
			}
		}
	}
	return false
}
