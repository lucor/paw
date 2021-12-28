package paw

import (
	"context"
	"encoding/gob"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
)

func init() {
	gob.Register((*Note)(nil))
}

// Declare conformity to Item interface
var _ Item = (*Note)(nil)

// Declare conformity to FyneObject interface
var _ FyneObject = (*Note)(nil)

type Note struct {
	Value string
	Metadata
}

func NewNote() *Note {
	return &Note{
		Metadata: Metadata{
			IconResource: icon.NoteOutlinedIconThemed,
			Type:         NoteItemType,
		},
	}
}

func (n *Note) Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, Item) {
	noteItem := *n
	titleEntry := widget.NewEntryWithData(binding.BindString(&noteItem.Metadata.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled note"

	noteEntry := widget.NewEntryWithData(binding.BindString(&noteItem.Value))
	noteEntry.MultiLine = true
	noteEntry.Validator = nil

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(n.Icon()))
	form.Add(titleEntry)
	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, &noteItem
}

func (n *Note) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(n.Icon(), n.Name)
	obj = append(obj, copiableRow("Note", n.Value, w)...)
	return container.New(
		layout.NewFormLayout(),
		obj...,
	)
}
