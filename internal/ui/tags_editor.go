// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"slices"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"go.lucor.dev/paw/internal/ui/widgets"
)

// tagsEditor edits the CXF tags assigned to one item.
type tagsEditor struct {
	View      *fyne.Container
	input     *widget.SelectEntry
	tags      []string
	initial   []string
	rows      *fyne.Container
	suggest   []string
	inputRow  *fyne.Container
	addButton *widget.Button
	canvas    fyne.Canvas
	escape    *tagEscapeShortcut
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
	e.input.SetPlaceHolder("Tag")
	e.input.OnSubmitted = func(string) { e.addTag() }
	e.input.OnChanged = func(value string) {
		if slices.Contains(e.suggest, value) {
			e.addTag()
		}
	}
	e.escape = &tagEscapeShortcut{}
	e.rows = container.NewVBox()
	e.inputRow = container.NewStack(e.input)
	e.inputRow.Hide()
	e.addButton = widget.NewButtonWithIcon("Add tag", icons.TagPlus, e.showAdd)
	e.addButton.Importance = widget.LowImportance
	e.View.Objects = []fyne.CanvasObject{e.rows}
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
func (e *tagsEditor) addTag() {
	tag := strings.TrimSpace(e.input.Text)
	if tag == "" || slices.Contains(e.tags, tag) {
		return
	}
	e.tags = append(e.tags, tag)
	e.closeInput()
	e.updateOptions()
	e.refresh()
}

// closeInput removes the temporary Escape shortcut and hides the input.
func (e *tagsEditor) closeInput() {
	if e.canvas != nil {
		e.canvas.RemoveShortcut(e.escape)
		e.canvas = nil
	}
	e.input.SetText("")
	e.inputRow.Hide()
}

// refresh rebuilds the tag rows and their remove actions.
func (e *tagsEditor) refresh() {
	rows := make([]fyne.CanvasObject, 0, len(e.tags))
	for index, tag := range e.tags {
		badge := newTagBadge(tag)
		badge.SetAction(func() {
			e.tags = append(e.tags[:index], e.tags[index+1:]...)
			e.updateOptions()
			e.refresh()
		})
		rows = append(rows, badge)
	}
	if !e.inputRow.Visible() {
		rows = append(rows, e.addButton)
	}
	objects := []fyne.CanvasObject{container.New(&tagWrapLayout{gap: theme.Size(theme.SizeNamePadding)}, rows...)}
	if e.inputRow.Visible() {
		objects = append(objects, e.inputRow)
	}
	e.rows.Objects = objects
	e.rows.Refresh()
	e.View.Refresh()
}

// newTagBadge builds a tag badge with Paw's neutral theme colors.
func newTagBadge(tag string) *widgets.Badge {
	badge := widgets.NewBadge(tag, pawColor(pawColorNameNeutralBadgeBackground), pawColor(pawColorNameNeutralBadgeForeground))
	badge.PaddingX, badge.PaddingY = 7, 3
	return badge
}

// showAdd reveals the temporary tag input.
func (e *tagsEditor) showAdd() {
	e.inputRow.Show()
	e.refresh()
	e.canvas = fyne.CurrentApp().Driver().CanvasForObject(e.input)
	if e.canvas != nil {
		e.canvas.AddShortcut(e.escape, func(fyne.Shortcut) { e.cancelAdd() })
		e.canvas.Focus(e.input)
	}
}

// cancelAdd closes the temporary input without changing the assigned tags.
func (e *tagsEditor) cancelAdd() {
	e.closeInput()
	e.refresh()
}

type tagEscapeShortcut struct{}

// Key returns Escape for the temporary tag input shortcut.
func (*tagEscapeShortcut) Key() fyne.KeyName { return fyne.KeyEscape }

// Mod reports that Escape has no modifiers.
func (*tagEscapeShortcut) Mod() fyne.KeyModifier { return 0 }

// ShortcutName identifies the temporary tag input shortcut.
func (*tagEscapeShortcut) ShortcutName() string { return "PawTagInputEscape" }

var _ fyne.KeyboardShortcut = (*tagEscapeShortcut)(nil)

// tagWrapLayout places chips on additional rows when the available width is limited.
type tagWrapLayout struct{ gap float32 }

// MinSize returns the largest chip size so the layout can wrap at its allocated width.
func (l *tagWrapLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
	var width, height float32
	for _, object := range objects {
		size := object.MinSize()
		if size.Width > width {
			width = size.Width
		}
		if size.Height > height {
			height = size.Height
		}
	}
	return fyne.NewSize(width, height)
}

// Layout places chips from left to right and starts a new row when needed.
func (l *tagWrapLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
	x, y, rowHeight := float32(0), float32(0), float32(0)
	for _, object := range objects {
		objectSize := object.MinSize()
		if x > 0 && x+objectSize.Width > size.Width {
			x = 0
			y += rowHeight + l.gap
			rowHeight = 0
		}
		object.Move(fyne.NewPos(x, y))
		object.Resize(objectSize)
		x += objectSize.Width + l.gap
		if objectSize.Height > rowHeight {
			rowHeight = objectSize.Height
		}
	}
}

var _ fyne.Layout = (*tagWrapLayout)(nil)

// Apply writes the edited tags to an item copy.
func (e *tagsEditor) Apply(item *model.Item) {
	item.Tags = slices.Clone(e.tags)
}

// IsDirty reports whether assigned tags changed.
func (e *tagsEditor) IsDirty() bool {
	return !slices.Equal(e.tags, e.initial)
}
