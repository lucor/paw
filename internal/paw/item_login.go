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
	gob.Register((*Login)(nil))
}

// Declare conformity to Item interface
var _ Item = (*Login)(nil)

// Declare conformity to FyneObject interface
var _ FyneObject = (*Login)(nil)

type Login struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`

	Username string `json:"username,omitempty"`
	URL      string `json:"url,omitempty"`
}

func NewLogin() *Login {
	return &Login{
		Metadata: &Metadata{
			Type: LoginItemType,
		},
		Note:     &Note{},
		Password: &Password{},
		TOTP:     &TOTP{},
	}
}

func (login *Login) Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, Item) {

	loginIcon := widget.NewIcon(login.Icon())

	loginItem := &Login{}
	*loginItem = *login
	loginItem.Metadata = &Metadata{}
	*loginItem.Metadata = *login.Metadata
	loginItem.Note = &Note{}
	*loginItem.Note = *login.Note
	loginItem.Password = &Password{}
	*loginItem.Password = *login.Password
	loginItem.TOTP = &TOTP{}
	*loginItem.TOTP = *login.TOTP

	passwordBind := binding.BindString(&loginItem.Password.Value)

	titleEntry := widget.NewEntryWithData(binding.BindString(&loginItem.Name))
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled login"

	urlEntry := newURLEntryWithData(ctx, binding.BindString(&loginItem.URL))
	urlEntry.TitleEntry = titleEntry
	urlEntry.FaviconListener = func(favicon fyne.Resource) {
		loginItem.Metadata.IconResource = favicon
		loginIcon.SetResource(favicon)
	}

	usernameEntry := widget.NewEntryWithData(binding.BindString(&loginItem.Username))
	usernameEntry.Validator = nil

	totpForm, totpItem := loginItem.TOTP.Edit(ctx, w)
	loginItem.TOTP = totpItem

	// the note field
	noteEntry := widget.NewEntryWithData(binding.BindString(&loginItem.Note.Value))
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
		login.Password.fpg.ShowPasswordGenerator(passwordBind, loginItem.Password, w)
	})

	form := container.New(layout.NewFormLayout())
	form.Add(loginIcon)
	form.Add(titleEntry)

	form.Add(labelWithStyle("URL"))
	form.Add(urlEntry)

	form.Add(labelWithStyle("Username"))
	form.Add(usernameEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewHBox(passwordCopyButton, passwordMakeButton), passwordEntry))

	form.Objects = append(form.Objects, totpForm.(*fyne.Container).Objects...)

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, loginItem
}

func (login *Login) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(login.Icon(), login.Name)
	if login.URL != "" {
		obj = append(obj, copiableLinkRow("URL", login.URL, w)...)
	}
	if login.Username != "" {
		obj = append(obj, copiableRow("Username", login.Username, w)...)
	}
	if login.Password.Value != "" {
		obj = append(obj, copiablePasswordRow("Password", login.Password.Value, w)...)
	}
	if login.TOTP != nil && login.TOTP.Secret != "" {
		obj = append(obj, login.TOTP.Show(ctx, w)...)
	}
	if login.Note != nil && login.Note.Value != "" {
		obj = append(obj, copiableRow("Note", login.Note.Value, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}

type urlEntry struct {
	widget.Entry
	TitleEntry      *widget.Entry
	FaviconListener func(fyne.Resource)

	ctx  context.Context
	host string // host keep track of the initial value before editing
}

func newURLEntryWithData(ctx context.Context, bind binding.String) *urlEntry {
	e := &urlEntry{
		ctx: ctx,
	}
	e.ExtendBaseWidget(e)
	e.Bind(bind)
	e.Validator = nil

	rawurl, _ := bind.Get()
	if rawurl == "" {
		rawurl = "https://"
		e.SetText(rawurl)
	}

	e.host = e.hostFromRawURL(rawurl)
	return e
}

func (e *urlEntry) hostFromRawURL(rawurl string) string {
	if rawurl == "" {
		return rawurl
	}

	u, err := url.Parse(rawurl)
	if err != nil {
		return rawurl
	}

	if u.Host != "" {
		return u.Host
	}
	parts := strings.Split(u.Path, "/")
	return parts[0]
}

// FocusLost is a hook called by the focus handling logic after this object lost the focus.
func (e *urlEntry) FocusLost() {
	defer e.Entry.FocusLost()

	host := e.hostFromRawURL(e.Text)
	if e.TitleEntry.Text == "" {
		e.TitleEntry.SetText(host)
	}
	if host == e.host {
		return
	}
	e.host = host

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
