// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package ui

import (
	"context"
	"fmt"
	"io"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/ssh"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// Declare conformity to FyneItem interface
var _ FyneItemWidget = (*sshItemWidget)(nil)

func NewSSHWidget(item *paw.SSHKey, preferences *paw.Preferences) FyneItemWidget {
	return &sshItemWidget{
		item:        item,
		preferences: preferences,
	}
}

type sshItemWidget struct {
	item        *paw.SSHKey
	preferences *paw.Preferences
	validator   []fyne.Validatable
}

func (iw *sshItemWidget) OnSubmit() (paw.Item, error) {
	for _, v := range iw.validator {
		if err := v.Validate(); err != nil {
			return nil, err
		}
	}

	iw.item.Metadata.Subtitle = iw.item.Subtitle()

	return iw.Item(), nil
}

func (iw *sshItemWidget) Item() paw.Item {
	copy := paw.NewSSHKey()
	err := deepCopyItem(iw.item, copy)
	if err != nil {
		panic(err)
	}
	return copy
}

func (iw *sshItemWidget) Icon() fyne.Resource {
	return icon.KeyOutlinedIconThemed
}

func (iw *sshItemWidget) Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject {
	titleEntryBind := binding.BindString(&iw.item.Name)
	titleEntry := widget.NewEntryWithData(titleEntryBind)
	titleEntry.Validator = requiredValidator("The title cannot be emtpy")
	titleEntry.PlaceHolder = "Untitled SSH Key"

	preferences := iw.preferences
	passphraseBind := binding.BindString(&iw.item.Passphrase.Value)
	passphraseEntry := widget.NewPasswordEntry()
	passphraseEntry.Bind(passphraseBind)
	passphraseEntry.Validator = nil
	passphraseEntry.SetPlaceHolder("Passphrase")

	passphraseActionMenu := []*fyne.MenuItem{
		{
			Label: "Generate",
			Icon:  icon.KeyOutlinedIconThemed,
			Action: func() {
				pg := NewPasswordGenerator(key, preferences.Password)
				pg.ShowPasswordGenerator(passphraseBind, iw.item.Passphrase, w)
			},
		},
		{
			Label: "Copy",
			Icon:  theme.ContentCopyIcon(),
			Action: func() {
				fyne.CurrentApp().Clipboard().SetContent(passphraseEntry.Text)
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "paw",
					Content: "Passphrase copied to clipboard",
				})
			},
		},
	}

	publicKeyEntryBind := binding.BindString(&iw.item.PublicKey)
	publicKeyEntry := widget.NewEntryWithData(publicKeyEntryBind)
	publicKeyEntry.Validator = nil
	publicKeyEntry.MultiLine = true
	publicKeyEntry.Wrapping = fyne.TextWrapBreak
	publicKeyEntry.Disable()

	publicKeyActionMenu := []*fyne.MenuItem{
		{
			Label: "Copy",
			Icon:  theme.ContentCopyIcon(),
			Action: func() {
				fyne.CurrentApp().Clipboard().SetContent(publicKeyEntry.Text)
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "paw",
					Content: "Public Key copied to clipboard",
				})
			},
		},
		{
			Label: "Export",
			Icon:  icon.DownloadOutlinedIconThemed,
			Action: func() {
				d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
					if uc == nil {
						// file open dialog has been cancelled
						return
					}
					if err != nil {
						dialog.NewError(err, w).Show()
						return
					}
					defer uc.Close()
					v, err := publicKeyEntryBind.Get()
					if err != nil {
						dialog.NewError(err, w).Show()
						return
					}
					uc.Write([]byte(v))
				}, w)
				filename, _ := titleEntryBind.Get()
				d.SetFileName(fmt.Sprintf("%s.pub", filename))
				d.Show()
			},
		},
	}

	fingerprintEntryBind := binding.BindString(&iw.item.Fingerprint)
	fingerprintEntry := widget.NewLabelWithData(fingerprintEntryBind)
	fingerprintEntry.Wrapping = fyne.TextWrapBreak

	privateKeyEntryBind := binding.BindString(&iw.item.PrivateKey)
	privateKeyEntry := widget.NewEntryWithData(privateKeyEntryBind)
	privateKeyEntry.Validator = nil
	privateKeyEntry.MultiLine = true
	privateKeyEntry.Wrapping = fyne.TextWrapBreak
	privateKeyEntry.Disable()
	privateKeyEntry.SetPlaceHolder("Private Key")

	privateKeyActionMenu := []*fyne.MenuItem{
		{
			Label: "Generate",
			Icon:  icon.KeyOutlinedIconThemed,
			Action: func() {
				sk, err := sshkey.GenerateKey()
				if err != nil {
					dialog.NewError(err, w).Show()
					return
				}
				privateKeyEntryBind.Set(string(sk.MarshalPrivateKey()))
				publicKeyEntryBind.Set(string(sk.MarshalPublicKey()))
				fingerprintEntryBind.Set(string(sk.Fingerprint()))
			},
		},
		{
			Label: "Copy",
			Icon:  theme.ContentCopyIcon(),
			Action: func() {
				fyne.CurrentApp().Clipboard().SetContent(privateKeyEntry.Text)
				fyne.CurrentApp().SendNotification(&fyne.Notification{
					Title:   "paw",
					Content: "Private Key copied to clipboard",
				})
			},
		},
		{
			Label: "Import",
			Icon:  icon.UploadOutlinedIconThemed,
			Action: func() {
				d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, e error) {
					b, err := io.ReadAll(uc)
					uc.Close()
					if err != nil {
						dialog.NewError(err, w).Show()
						return
					}
					sk, err := sshkey.ParseKey(b)
					if err != nil {
						if _, ok := err.(*ssh.PassphraseMissingError); !ok {
							dialog.NewError(err, w).Show()
							return
						}
						passphraseEntry := widget.NewPasswordEntry()
						content := widget.NewFormItem("", passphraseEntry)
						a := dialog.NewForm("Private Key is password protected", "Confirm", "Cancel",
							[]*widget.FormItem{content},
							func(isConfirm bool) {
								if !isConfirm {
									return
								}
								passphrase := passphraseEntry.Text
								sk, err = sshkey.ParseKeyWithPassphrase(b, []byte(passphrase))
								if err != nil {
									dialog.NewError(err, w).Show()
									return
								}
								passphraseBind.Set(passphrase)
								privateKeyEntryBind.Set(string(sk.MarshalPrivateKey()))
								publicKeyEntryBind.Set(string(sk.MarshalPublicKey()))
								fingerprintEntryBind.Set(string(sk.Fingerprint()))
							},
							w,
						)
						a.Show()
						return
					}
					privateKeyEntryBind.Set(string(sk.MarshalPrivateKey()))
					publicKeyEntryBind.Set(string(sk.MarshalPublicKey()))
					fingerprintEntryBind.Set(string(sk.Fingerprint()))
				}, w)
				d.Show()
			},
		},
		{
			Label: "Export",
			Icon:  icon.DownloadOutlinedIconThemed,
			Action: func() {
				d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
					if uc == nil {
						// file open dialog has been cancelled
						return
					}
					if err != nil {
						dialog.NewError(err, w).Show()
						return
					}
					defer uc.Close()
					v, err := privateKeyEntryBind.Get()
					if err != nil {
						dialog.NewError(err, w).Show()
						return
					}
					uc.Write([]byte(v))
				}, w)
				filename, _ := titleEntryBind.Get()
				d.SetFileName(filename)
				d.Show()
			},
		},
	}

	commentEntryBind := binding.BindString(&iw.item.Comment)
	commentEntry := widget.NewEntryWithData(commentEntryBind)
	commentEntry.Validator = nil
	commentEntry.PlaceHolder = "Public Key Comment"

	addToAgentCheckBind := binding.BindBool(&iw.item.AddToAgent)
	addToAgentCheck := widget.NewCheckWithData("", addToAgentCheckBind)

	noteEntry := newNoteEntryWithData(binding.BindString(&iw.item.Note.Value))

	iw.validator = append(iw.validator, titleEntry)

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(iw.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Private Key"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(privateKeyActionMenu, w)), privateKeyEntry))

	form.Add(labelWithStyle("Passphrase"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(passphraseActionMenu, w)), passphraseEntry))

	form.Add(labelWithStyle("Comment"))
	form.Add(commentEntry)

	form.Add(labelWithStyle("Public Key"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(publicKeyActionMenu, w)), publicKeyEntry))

	form.Add(labelWithStyle("Fingerprint"))
	form.Add(fingerprintEntry)

	form.Add(labelWithStyle("Add to SSH Agent"))
	form.Add(addToAgentCheck)

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form
}

func (iw *sshItemWidget) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(iw.Icon(), iw.item.Name)
	obj = append(obj, rowWithAction("Private Key", iw.item.PrivateKey, rowActionOptions{copy: true, ellipsis: 64, export: iw.item.Name}, w)...)
	if iw.item.Passphrase != nil && iw.item.Passphrase.Value != "" {
		obj = append(obj, rowWithAction("Passphrase", iw.item.Passphrase.Value, rowActionOptions{widgetType: "password", copy: true}, w)...)
	}
	if iw.item.Comment != "" {
		obj = append(obj, rowWithAction("Comment", iw.item.Comment, rowActionOptions{copy: true}, w)...)
	}
	obj = append(obj, rowWithAction("Public Key", iw.item.PublicKey, rowActionOptions{copy: true, ellipsis: 64, export: iw.item.Name + ".pub"}, w)...)
	obj = append(obj, rowWithAction("Fingerprint", iw.item.Fingerprint, rowActionOptions{copy: true}, w)...)
	if iw.item.Note.Value != "" {
		obj = append(obj, rowWithAction("Note", iw.item.Note.Value, rowActionOptions{copy: true}, w)...)
	}
	v := "No"
	if iw.item.AddToAgent {
		v = "Yes"
	}
	obj = append(obj, rowWithAction("Add to SSH Agent", v, rowActionOptions{}, w)...)
	return container.New(layout.NewFormLayout(), obj...)
}
