// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

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

// Declare conformity to FyneItem interface
var _ FyneItemWidget = (*noteItemWidget)(nil)

func NewNoteWidget(item *paw.Note) FyneItemWidget {
	return &noteItemWidget{
		item: item,
	}
}

type noteItemWidget struct {
	item *paw.Note

	validator []fyne.Validatable
}

// OnSubmit implements FyneItem.
func (iw *noteItemWidget) OnSubmit() (paw.Item, error) {
	for _, v := range iw.validator {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}
	return iw.Item(), nil
}

func (iw *noteItemWidget) Item() paw.Item {
	copy := paw.NewNote()
	err := deepCopyItem(iw.item, copy)
	if err != nil {
		panic(err)
	}
	return copy
}

func (iw *noteItemWidget) Icon() fyne.Resource {
	return icon.NoteOutlinedIconThemed
}

func (iw *noteItemWidget) Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject {
	titleEntry := widget.NewEntryWithData(binding.BindString(&iw.item.Metadata.Name))
	titleEntry.Validator = requiredValidator("The title cannot be emtpy")
	titleEntry.PlaceHolder = "Untitled note"

	noteEntry := newNoteEntryWithData(binding.BindString(&iw.item.Value))

	titleEntry.Validate()

	iw.validator = append(iw.validator, titleEntry)

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(iw.Icon()))
	form.Add(titleEntry)
	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form
}

func (iw *noteItemWidget) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	if iw == nil {
		return container.New(layout.NewFormLayout(), widget.NewLabel(""))
	}
	obj := titleRow(iw.Icon(), iw.item.Name)
	obj = append(obj, rowWithAction("Note", iw.item.Value, rowActionOptions{copy: true}, w)...)
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
