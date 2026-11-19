// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// tagsEditor edits the CXF tags assigned to one item.
type tagsEditor struct {
	View    *fyne.Container
	input   *widget.SelectEntry
	tags    []string
	initial []string
	rows    *fyne.Container
	suggest []string
}

// newTagsEditor builds a tag editor from an item and vault summaries.
func newTagsEditor(item *model.Item, suggestions []string) *tagsEditor {
	e := &tagsEditor{
		View:    container.NewVBox(),
		input:   widget.NewSelectEntry(nil),
		tags:    slices.Clone(item.Tags),
		initial: slices.Clone(item.Tags),
	}
	e.SetSuggestions(suggestions)
	e.input.SetPlaceHolder("Add a tag")
	e.rows = container.NewVBox()
	add := widget.NewButton("Add", e.add)
	add.Importance = widget.LowImportance
	e.View.Objects = []fyne.CanvasObject{e.rows, container.NewBorder(nil, nil, nil, add, e.input)}
	e.refresh()
	return e
}

// SetSuggestions updates the tag options from the current vault summaries.
func (e *tagsEditor) SetSuggestions(suggestions []string) {
	e.suggest = slices.Clone(suggestions)
	e.updateOptions()
}

// updateOptions refreshes suggestions that are not already assigned.
func (e *tagsEditor) updateOptions() {
	options := make([]string, 0, len(e.suggest))
	for _, suggestion := range e.suggest {
		if suggestion != "" && !slices.Contains(e.tags, suggestion) {
			options = append(options, suggestion)
		}
	}
	e.input.SetOptions(options)
}

// add appends the entered tag when it is non-empty and not already present.
func (e *tagsEditor) add() {
	tag := strings.TrimSpace(e.input.Text)
	if tag == "" || slices.Contains(e.tags, tag) {
		return
	}
	e.tags = append(e.tags, tag)
	e.input.SetText("")
	e.updateOptions()
	e.refresh()
}

// refresh rebuilds the tag rows and their remove actions.
func (e *tagsEditor) refresh() {
	rows := make([]fyne.CanvasObject, 0, len(e.tags))
	for index, tag := range e.tags {
		index, tag := index, tag
		remove := newIconButton("Remove tag", icons.Delete, func() {
			e.tags = append(e.tags[:index], e.tags[index+1:]...)
			e.updateOptions()
			e.refresh()
		})
		rows = append(rows, container.NewBorder(nil, nil, nil, remove, newText(tag, bodyStyle)))
	}
	e.rows.Objects = rows
	e.rows.Refresh()
	e.View.Refresh()
}

// Apply writes the edited tags to an item copy.
func (e *tagsEditor) Apply(item *model.Item) {
	item.Tags = slices.Clone(e.tags)
}

// IsDirty reports whether assigned tags changed.
func (e *tagsEditor) IsDirty() bool {
	return !slices.Equal(e.tags, e.initial)
}
