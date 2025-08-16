// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/favicon"
	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to FyneItemWidget interface
var _ FyneItemWidget = (*loginItemWidget)(nil)

func NewLoginWidget(item *paw.Login, preferences *paw.Preferences) FyneItemWidget {
	return &loginItemWidget{
		item:        item,
		preferences: preferences,
		urlEntry:    newURLEntryWithData(context.TODO(), item.URL, preferences.FaviconDownloader),
	}
}

// loginItemWidget handle a paw.Login as Fyne Widget
type loginItemWidget struct {
	item        *paw.Login
	preferences *paw.Preferences
	urlEntry    *urlEntry

	validator []fyne.Validatable
}

// OnSubmit implements FyneItem.
func (iw *loginItemWidget) OnSubmit() (paw.Item, error) {
	iw.urlEntry.FocusLost()

	for _, v := range iw.validator {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}

	if iw.item.Metadata.Autofill == nil {
		iw.item.Metadata.Autofill = &paw.Autofill{}
	}

	iw.item.Metadata.Autofill.URL = iw.urlEntry.loginURL.URL()
	iw.item.Metadata.Autofill.TLDPlusOne = iw.urlEntry.loginURL.TLDPlusOne()
	iw.item.Metadata.Subtitle = iw.item.Subtitle()

	return iw.Item(), nil
}

func (iw *loginItemWidget) Item() paw.Item {
	copy := paw.NewLogin()
	err := deepCopyItem(iw.item, copy)
	if err != nil {
		panic(err)
	}
	return copy
}

func (iw *loginItemWidget) Icon() fyne.Resource {
	if iw.item.Favicon != nil {
		return iw.item.Favicon
	}
	return icon.WorldWWWOutlinedIconThemed
}

func (iw *loginItemWidget) Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject {
	loginIcon := canvas.NewImageFromResource(iw.Icon())
	loginIcon.FillMode = canvas.ImageFillContain
	loginIcon.SetMinSize(fyne.NewSize(32, 32))

	if iw.item.URL == nil {
		iw.item.URL = paw.NewLoginURL()
	}
	preferences := iw.preferences

	passwordBind := binding.BindString(&iw.item.Password.Value)

	titleEntry := widget.NewEntryWithData(binding.BindString(&iw.item.Name))
	titleEntry.Validator = requiredValidator("The title cannot be emtpy")
	titleEntry.PlaceHolder = "Untitled login"

	urlEntry := newURLEntryWithData(ctx, iw.item.URL, preferences.FaviconDownloader)
	urlEntry.TitleEntry = titleEntry
	urlEntry.FaviconListener = func(favicon *paw.Favicon) {
		iw.item.Metadata.Favicon = favicon
		if favicon != nil {
			loginIcon.Resource = favicon
			loginIcon.Refresh()
			return
		}
		// no favicon found, fallback to default
		loginIcon.Resource = icon.WorldWWWOutlinedIconThemed
		loginIcon.Refresh()
	}
	iw.urlEntry = urlEntry

	usernameEntry := widget.NewEntryWithData(binding.BindString(&iw.item.Username))
	usernameEntry.Validator = nil

	uiTOTP := &TOTP{TOTP: iw.item.TOTP}
	totpForm, totpItem := uiTOTP.Edit(ctx, w)
	iw.item.TOTP = totpItem

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
				pg.ShowPasswordGenerator(passwordBind, iw.item.Password, w)
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

	iw.validator = append(iw.validator, titleEntry)

	form := container.New(layout.NewFormLayout())
	form.Add(container.NewCenter(loginIcon))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Website"))
	form.Add(urlEntry)

	form.Add(labelWithStyle("Username"))
	form.Add(usernameEntry)

	form.Add(labelWithStyle("Password"))

	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(passwordActionMenu, w)), passwordEntry))

	form.Objects = append(form.Objects, totpForm.(*fyne.Container).Objects...)

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form
}

func (iw *loginItemWidget) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(iw.Icon(), iw.item.Name)
	if iw.item.URL != nil {
		obj = append(obj, rowWithAction("Website", iw.item.URL.String(), rowActionOptions{widgetType: "url", copy: true}, w)...)
	}
	if iw.item.Username != "" {
		obj = append(obj, rowWithAction("Username", iw.item.Username, rowActionOptions{copy: true}, w)...)
	}
	if iw.item.Password.Value != "" {
		obj = append(obj, rowWithAction("Password", iw.item.Password.Value, rowActionOptions{widgetType: "password", copy: true}, w)...)
	}
	if iw.item.TOTP != nil && iw.item.TOTP.Secret != "" {
		uiTOTP := &TOTP{TOTP: iw.item.TOTP}
		obj = append(obj, uiTOTP.Show(ctx, w)...)
	}
	if iw.item.Note != nil && iw.item.Note.Value != "" {
		obj = append(obj, rowWithAction("Note", iw.item.Note.Value, rowActionOptions{copy: true}, w)...)
	}
	return container.New(layout.NewFormLayout(), obj...)
}

type urlEntry struct {
	widget.Entry
	TitleEntry      *widget.Entry
	FaviconListener func(*paw.Favicon)
	ctx             context.Context
	loginURL        *paw.LoginURL // keep track of the initial value before editing
	preferences     paw.FaviconDownloaderPreferences
	validationError error
}

func newURLEntryWithData(ctx context.Context, loginURL *paw.LoginURL, preferences paw.FaviconDownloaderPreferences) *urlEntry {
	e := &urlEntry{
		ctx:         ctx,
		loginURL:    loginURL,
		preferences: preferences,
	}
	e.ExtendBaseWidget(e)
	if e.loginURL != nil {
		e.SetText(e.loginURL.String())
	}
	e.Validator = func(s string) error {
		return e.validationError
	}
	e.OnSubmitted = func(s string) {
		e.FocusLost()
	}
	return e
}

func (e *urlEntry) FocusGained() {
	defer e.Entry.FocusGained()
	e.validationError = nil
}

// FocusLost is a hook called by the focus handling logic after this object lost the focus.
func (e *urlEntry) FocusLost() {
	defer e.Entry.FocusLost()

	oldHostname := e.loginURL.URL().Hostname()

	err := e.loginURL.Set(e.Text)
	if err != nil {
		e.validationError = err
		return
	}

	if e.Text != e.loginURL.String() {
		// update the text to the normalized URL, if it changed
		e.SetText(e.loginURL.String())
	}

	newHostname := e.loginURL.URL().Hostname()
	if e.TitleEntry.Text == "" {
		e.TitleEntry.SetText(newHostname)
	}

	if oldHostname == newHostname {
		// Host did not change, skipping favicon download
		return
	}

	if e.preferences.Disabled {
		// Favicons are disabled, skipping download
		e.FaviconListener(nil)
		return
	}

	go func() {
		var fav *paw.Favicon

		b, format, err := favicon.Download(e.ctx, e.loginURL.URL(), favicon.Options{})
		if err != nil {
			fyne.Do(func() {
				e.FaviconListener(fav)
			})
			return
		}

		fav = paw.NewFavicon(newHostname, b, format)
		fyne.Do(func() {
			e.FaviconListener(fav)
		})
	}()
}
