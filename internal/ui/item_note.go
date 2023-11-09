package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to Item interface
var _ paw.Item = (*Note)(nil)

// Declare conformity to FyneItem interface
var _ FyneItem = (*Note)(nil)

type Note struct {
	*paw.Note
}

func (n *Note) Item() paw.Item {
	return n.Note
}

func (n *Note) Icon() fyne.Resource {
	if n.Favicon != nil {
		return n.Favicon
	}
	return icon.NoteOutlinedIconThemed
}

func (n *Note) Edit(ctx context.Context, key *paw.Key, w fyne.Window) (fyne.CanvasObject, paw.Item) {
	item := &paw.Note{}
	*item = *n.Note
	item.Metadata = &paw.Metadata{}
	*item.Metadata = *n.Metadata

	titleEntry := widget.NewEntryWithData(binding.BindString(&item.Metadata.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled note"

	noteEntry := newNoteEntryWithData(binding.BindString(&item.Value))

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(n.Icon()))
	form.Add(titleEntry)
	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, item
}

func (n *Note) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	if n == nil {
		return container.New(layout.NewFormLayout(), widget.NewLabel(""))
	}
	obj := titleRow(n.Icon(), n.Name)
	obj = append(obj, rowWithAction("Note", n.Value, rowActionOptions{copy: true}, w)...)
	return container.New(layout.NewFormLayout(), obj...)
}

// noteEntry is a multiline entry widget that does not accept tab
// This will allow to change the widget focus when tab is pressed
type noteEntry struct {
	widget.Entry
}

func newNoteEntryWithData(bind binding.String) *noteEntry {
	ne := &noteEntry{
		Entry: widget.Entry{
			MultiLine: true,
			Wrapping:  fyne.TextWrap(fyne.TextTruncateEllipsis),
		},
	}
	ne.ExtendBaseWidget(ne)
	ne.Bind(bind)
	ne.Validator = nil
	return ne
}

// AcceptsTab returns if Entry accepts the Tab key or not.
//
// Implements: fyne.Tabbable
func (ne *noteEntry) AcceptsTab() bool {
	return false
}
