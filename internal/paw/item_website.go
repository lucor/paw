package paw

import (
	"bytes"
	"context"
	"encoding/gob"
	"image/png"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/favicon"
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

	websiteIcon := widget.NewIcon(website.Icon())

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

	websiteEntry := newWebsiteEntryWithData(ctx, binding.BindString(&websiteItem.URI))
	websiteEntry.FaviconListener = func(favicon fyne.Resource) {
		websiteItem.Metadata.IconResource = favicon
		websiteIcon.SetResource(favicon)
	}

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
	form.Add(websiteIcon)
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
	obj = append(obj, copiableLinkRow("Website", website.URI, w)...)
	obj = append(obj, copiableRow("Username", website.Username, w)...)
	obj = append(obj, copiablePasswordRow("Password", website.Password.Value, w)...)
	if website.TOTP != nil && website.TOTP.Secret != "" {
		obj = append(obj, website.TOTP.Show(ctx, w)...)
	}
	if website.Note != nil && website.Note.Value != "" {
		obj = append(obj, copiableRow("Note", website.Note.Value, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}

type websiteEntry struct {
	ctx context.Context
	widget.Entry
	host            string // host keep track of the initial value before editing
	BaseURL         string
	FaviconListener func(fyne.Resource)
}

func newWebsiteEntryWithData(ctx context.Context, bind binding.String) *websiteEntry {
	e := &websiteEntry{
		ctx: ctx,
	}
	e.ExtendBaseWidget(e)
	e.Bind(bind)
	e.Validator = func(s string) error {
		rawurl, _ := bind.Get()
		e.host = ""
		u, err := url.Parse(rawurl)
		if err != nil {
			return err
		}

		if strings.HasPrefix(u.Scheme, "http") {
			e.host = u.Host
		}
		return nil
	}
	return e
}

// FocusLost is a hook called by the focus handling logic after this object lost the focus.
func (e *websiteEntry) FocusLost() {
	defer e.Entry.FocusLost()

	host := e.host
	if host == "" {
		return
	}

	go func() {
		var resource fyne.Resource
		resource = icon.PublicOutlinedIconThemed

		img, err := favicon.Download(e.ctx, host, favicon.Options{
			ForceMinSize: true,
		})
		if err != nil {
			e.FaviconListener(resource)
			return
		}

		w := &bytes.Buffer{}
		err = png.Encode(w, img)
		if err != nil {
			e.FaviconListener(resource)
			return
		}
		resource = fyne.NewStaticResource(host, w.Bytes())
		e.FaviconListener(resource)
	}()
}
