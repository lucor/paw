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

// Declare conformity to Item interface
var _ paw.Item = (*Password)(nil)

// Declare conformity to FyneItem interface
var _ FyneItem = (*Password)(nil)

type Password struct {
	*paw.Password
}

func (p *Password) Item() paw.Item {
	return p.Password
}

func (p *Password) Icon() fyne.Resource {
	if p.Favicon != nil {
		return p.Favicon
	}
	return icon.PasswordOutlinedIconThemed
}

func (p *Password) Edit(ctx context.Context, key *paw.Key, w fyne.Window) (fyne.CanvasObject, paw.Item) {
	passwordItem := &paw.Password{}
	*passwordItem = *p.Password
	passwordItem.Metadata = &paw.Metadata{}
	*passwordItem.Metadata = *p.Metadata
	passwordItem.Note = &paw.Note{}
	*passwordItem.Note = *p.Note

	passwordBind := binding.BindString(&passwordItem.Value)
	titleEntry := widget.NewEntryWithData(binding.BindString(&passwordItem.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled password"

	// the note field
	noteEntry := widget.NewEntryWithData(binding.BindString(&passwordItem.Note.Value))
	noteEntry.MultiLine = true
	noteEntry.Validator = nil

	// center
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.Bind(passwordBind)
	passwordEntry.Validator = nil
	passwordEntry.SetPlaceHolder("Password")

	passwordCopyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(passwordEntry.Text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: "Password copied to clipboard",
		})
	})

	passwordMakeButton := widget.NewButtonWithIcon("Generate", icon.KeyOutlinedIconThemed, func() {
		pg := NewPasswordGenerator(key)
		pg.ShowPasswordGenerator(passwordBind, passwordItem, w)
	})

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(p.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewHBox(passwordCopyButton, passwordMakeButton), passwordEntry))

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, passwordItem
}

func (p *Password) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(p.Icon(), p.Name)
	if p.Value != "" {
		obj = append(obj, copiablePasswordRow("Password", p.Value, w)...)
	}
	if p.Note.Value != "" {
		obj = append(obj, copiableRow("Note", p.Note.Value, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}
