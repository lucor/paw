package paw

import (
	"context"
	"encoding/gob"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
)

func init() {
	gob.Register((*Website)(nil))
}

// Declare conformity to Item interface
var _ Item = (*Website)(nil)

// Declare conformity to FyneObject interface
var _ FyneObject = (*Website)(nil)

type Website struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`

	Username string `json:"username,omitempty"`
	URI      string `json:"uri,omitempty"`
}

func NewWebsite() *Website {
	return &Website{
		Metadata: &Metadata{
			Type: WebsiteItemType,
		},
		Note:     &Note{},
		Password: &Password{},
		TOTP:     &TOTP{},
	}
}

func (website *Website) Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, Item) {
	websiteItem := &Website{}
	*websiteItem = *website
	websiteItem.Metadata = &Metadata{}
	*websiteItem.Metadata = *website.Metadata
	websiteItem.Note = &Note{}
	*websiteItem.Note = *website.Note
	websiteItem.Password = &Password{}
	*websiteItem.Password = *website.Password
	websiteItem.TOTP = &TOTP{}
	*websiteItem.TOTP = *website.TOTP

	passwordBind := binding.BindString(&websiteItem.Password.Value)
	titleEntry := widget.NewEntryWithData(binding.BindString(&websiteItem.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled website"

	websiteEntry := widget.NewEntryWithData(binding.BindString(&websiteItem.URI))
	websiteEntry.Validator = nil

	usernameEntry := widget.NewEntryWithData(binding.BindString(&websiteItem.Username))
	usernameEntry.Validator = nil

	totpForm, totpItem := websiteItem.TOTP.Edit(ctx, w)
	websiteItem.TOTP = totpItem

	// the note field
	noteEntry := widget.NewEntryWithData(binding.BindString(&websiteItem.Note.Value))
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
		website.Password.fpg.ShowPasswordGenerator(passwordBind, websiteItem.Password, w)
	})

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(website.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Website"))
	form.Add(websiteEntry)

	form.Add(labelWithStyle("Username"))
	form.Add(usernameEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewHBox(passwordCopyButton, passwordMakeButton), passwordEntry))

	form.Objects = append(form.Objects, totpForm.(*fyne.Container).Objects...)

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, websiteItem
}

func (website *Website) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(website.Icon(), website.Name)
	obj = append(obj, copiableRow("Website", website.URI, w)...)
	obj = append(obj, copiableRow("Username", website.Username, w)...)
	obj = append(obj, copiablePasswordRow("Password", website.Password.Value, w)...)
	if website.TOTP.Secret != "" {
		obj = append(obj, website.TOTP.Show(ctx, w)...)
	}
	if website.Note.Value != "" {
		obj = append(obj, copiableRow("Note", website.Note.Value, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}
