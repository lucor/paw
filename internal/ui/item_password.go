// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to FyneItem interface
var _ FyneItemWidget = (*passwordItemWidget)(nil)

func NewPasswordWidget(item *paw.Password, preferences *paw.Preferences) FyneItemWidget {
	return &passwordItemWidget{
		item:        item,
		preferences: preferences,
	}
}

type passwordItemWidget struct {
	item        *paw.Password
	preferences *paw.Preferences
	validator   []fyne.Validatable
}

func (iw *passwordItemWidget) Item() paw.Item {
	copy := paw.NewPassword()
	err := deepCopyItem(iw.item, copy)
	if err != nil {
		panic(err)
	}
	return copy
}

func (iw *passwordItemWidget) Icon() fyne.Resource {
	return icon.PasswordOutlinedIconThemed
}

// OnSubmit implements FyneItem.
func (iw *passwordItemWidget) OnSubmit() (paw.Item, error) {
	for _, v := range iw.validator {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}
	return iw.Item(), nil
}

func (iw *passwordItemWidget) Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject {
	preferences := iw.preferences

	passwordBind := binding.BindString(&iw.item.Value)
	titleEntry := widget.NewEntryWithData(binding.BindString(&iw.item.Name))
	titleEntry.Validator = requiredValidator("The title cannot be emtpy")
	titleEntry.PlaceHolder = "Untitled password"

	// the note field
	noteEntry := newNoteEntryWithData(binding.BindString(&iw.item.Note.Value))

	// center
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.Bind(passwordBind)
	passwordEntry.Validator = nil
	passwordEntry.SetPlaceHolder("Password")

	passwordActionMenu := []*fyne.MenuItem{
		{
			Label: "Generate",
			Icon:  icon.KeyOutlinedIconThemed,
			Action: func() {
				pg := NewPasswordGenerator(key, preferences.Password)
				pg.ShowPasswordGenerator(passwordBind, iw.item, w)
			},
		},
		{
			Label: "Copy",
			Icon:  theme.ContentCopyIcon(),
			Action: func() {
				fyne.CurrentApp().Clipboard().SetContent(passwordEntry.Text)
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "paw",
					Content: "Password copied to clipboard",
				})
			},
		},
	}

	iw.validator = append(iw.validator, titleEntry, passwordEntry)

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(iw.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(passwordActionMenu, w)), passwordEntry))

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form
}

func (iw *passwordItemWidget) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(iw.Icon(), iw.item.Name)
	if iw.item.Value != "" {
		obj = append(obj, rowWithAction("Password", iw.item.Value, rowActionOptions{widgetType: "password", copy: true}, w)...)
	}
	if iw.item.Note.Value != "" {
		obj = append(obj, rowWithAction("Note", iw.item.Note.Value, rowActionOptions{copy: true}, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}
