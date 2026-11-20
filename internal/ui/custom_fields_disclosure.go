// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// customFieldsDisclosure hides or shows an item's custom-fields editor.
type customFieldsDisclosure struct {
	View      *fyne.Container
	content   *fyne.Container
	heading   *widget.Button
	header    *fyne.Container
	add       *iconButton
	count     int
	populated bool
	expanded  bool
}

// newCustomFieldsDisclosure shows the add actions directly when there are no fields or sections.
func newCustomFieldsDisclosure(editor *customFieldsEditor) *customFieldsDisclosure {
	d := &customFieldsDisclosure{
		View:      container.NewVBox(),
		content:   container.NewVBox(editor.View),
		count:     editor.fieldCount(),
		populated: editor.hasSections() || editor.fieldCount() > 0,
		expanded:  true,
	}
	d.heading = widget.NewButtonWithIcon("", theme.Icon(theme.IconNameArrowDropDown), d.toggle)
	d.heading.Alignment = widget.ButtonAlignLeading
	d.heading.Importance = widget.LowImportance
	d.heading.SetText("Custom fields")
	d.add = customFieldsMenu("Add custom fields",
		fyne.NewMenuItem("Add field", func() { editor.addField(-1) }),
		fyne.NewMenuItem("Add section", func() { editor.showSectionDialog(-1) }),
	)
	d.add.SetIcon(icons.Plus)
	background := canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground))
	background.CornerRadius = theme.Size(theme.SizeNamePadding)
	d.header = container.NewStack(background, d.heading)
	editor.onAdd = func() { d.expanded = true }
	editor.onChange = func(count int) {
		populated := count > 0 || editor.hasSections()
		if populated && !d.populated {
			d.expanded = true
		} else if !populated && d.populated {
			d.expanded = true
		}
		d.count = count
		d.populated = populated
		d.refresh()
	}
	d.refresh()
	return d
}

// toggle changes the local expansion state.
func (d *customFieldsDisclosure) toggle() {
	d.expanded = !d.expanded
	d.refresh()
}

// refresh updates the heading and empty-state action.
func (d *customFieldsDisclosure) refresh() {
	heading := "Custom fields"
	if d.count > 0 {
		heading += " · " + strconv.Itoa(d.count) + " fields"
	}
	d.heading.SetText(heading)
	icon := theme.Icon(theme.IconNameArrowDropDown)
	if d.expanded {
		icon = theme.Icon(theme.IconNameArrowDropUp)
		d.content.Show()
	} else {
		d.content.Hide()
	}
	d.heading.SetIcon(icon)
	row := fyne.CanvasObject(d.heading)
	if d.populated {
		row = container.NewBorder(nil, nil, nil, d.add, d.heading)
	}
	d.header.Objects = []fyne.CanvasObject{canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground)), row}
	d.View.Objects = []fyne.CanvasObject{d.header, d.content}
	d.View.Refresh()
}
