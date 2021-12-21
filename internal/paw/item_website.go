package paw

import (
	"context"
	"encoding/gob"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
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
	Metadata
	*Password
	*TOTP

	Username string
	Website  string
}

func NewWebsite(password *Password, totp *TOTP) *Website {
	return &Website{
		Password: password,
		TOTP:     totp,
	}
}

func (website *Website) ID() string {
	return fmt.Sprintf("website/%s", strings.ToLower(website.Title))
}

func (website *Website) Icon() *widget.Icon {
	return widget.NewIcon(icon.PublicOutlinedIconThemed)
}

func (website *Website) Type() ItemType {
	return WebsiteItemType
}

func (website *Website) Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, Item) {
	item := *website
	if item.TOTP == nil {
		item.TOTP = NewDefaultTOTP()
	}
	passwordBind := binding.BindString(&item.Password.Password)
	titleEntry := widget.NewEntryWithData(binding.BindString(&item.Title))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled website"

	websiteEntry := widget.NewEntryWithData(binding.BindString(&item.Website))
	websiteEntry.Validator = nil

	usernameEntry := widget.NewEntryWithData(binding.BindString(&item.Username))
	usernameEntry.Validator = nil

	totpForm, totpItem := item.TOTP.Edit(ctx, w)
	item.TOTP = totpItem

	// the note field
	noteEntry := widget.NewEntryWithData(binding.BindString(&item.Note))
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
		copy := item
		d := dialog.NewCustomConfirm("Generate password", "Use", "Cancel", copy.makePasswordDialog(), func(b bool) {
			if b {
				passwordBind.Set(copy.Password.Password)
			}
		}, w)
		d.Show()
	})

	form := container.New(layout.NewFormLayout())
	form.Add(website.Icon())
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

	return form, &item
}

func (website *Website) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(website.Icon(), website.Title)
	obj = append(obj, copiableRow("Website", website.Website, w)...)
	obj = append(obj, copiableRow("Username", website.Username, w)...)
	obj = append(obj, copiablePasswordRow("Password", website.Password.Password, w)...)
	if website.TOTP != nil && website.TOTP.Secret != "" {
		obj = append(obj, website.TOTP.Show(ctx, w)...)
	}
	obj = append(obj, copiableRow("Note", website.Note, w)...)
	return container.New(layout.NewFormLayout(), obj...)
}
