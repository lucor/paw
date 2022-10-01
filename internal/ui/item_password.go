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
	Config *paw.Config
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

	config := p.Config

	passwordBind := binding.BindString(&passwordItem.Value)
	titleEntry := widget.NewEntryWithData(binding.BindString(&passwordItem.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled password"

	// the note field
	noteEntry := newNoteEntryWithData(binding.BindString(&passwordItem.Note.Value))

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
				pg := NewPasswordGenerator(key, config.Password)
				pg.ShowPasswordGenerator(passwordBind, passwordItem, w)
			},
		},
		{
			Label: "Copy",
			Icon:  theme.ContentCopyIcon(),
			Action: func() {
				w.Clipboard().SetContent(passwordEntry.Text)
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "paw",
					Content: "Password copied to clipboard",
				})
			},
		},
	}

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(p.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(passwordActionMenu, w)), passwordEntry))

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, passwordItem
}

func (p *Password) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(p.Icon(), p.Name)
	if p.Value != "" {
		obj = append(obj, rowWithAction("Password", p.Value, rowActionOptions{widgetType: "password", copy: true}, w)...)
	}
	if p.Note.Value != "" {
		obj = append(obj, rowWithAction("Note", p.Note.Value, rowActionOptions{copy: true}, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}
