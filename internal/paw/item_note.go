package paw

import (
	"encoding/gob"
	"fmt"
	"strings"

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
	Metadata
}

func NewNote() *Note {
	return &Note{}
}

func (n *Note) ID() string {
	return fmt.Sprintf("note/%s", strings.ToLower(n.Title))
}

func (n *Note) Icon() *widget.Icon {
	return widget.NewIcon(icon.NoteOutlinedIconThemed)
}

func (n *Note) Type() ItemType {
	return NoteItemType
}

func (n *Note) Edit(w fyne.Window) (fyne.CanvasObject, Item) {
	item := *n
	titleEntry := widget.NewEntryWithData(binding.BindString(&item.Title))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled note"

	noteEntry := widget.NewEntryWithData(binding.BindString(&item.Note))
	noteEntry.MultiLine = true
	noteEntry.Validator = nil

	form := container.New(layout.NewFormLayout())
	form.Add(n.Icon())
	form.Add(titleEntry)
	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, &item
}

func (n *Note) Show(w fyne.Window) fyne.CanvasObject {
	obj := titleRow(n.Icon(), n.Title)
	obj = append(obj, copiableRow("Note", n.Note, w)...)
	return container.New(
		layout.NewFormLayout(),
		obj...,
	)
}
