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

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
)

// Declare conformity to Item interface
var _ paw.Item = (*Password)(nil)

// Declare conformity to FyneItem interface
var _ FyneItem = (*Password)(nil)

type SSHKey struct {
	*paw.SSHKey
}

func (sh *SSHKey) Item() paw.Item {
	return sh.SSHKey
}

func (sh *SSHKey) Icon() fyne.Resource {
	if sh.Favicon != nil {
		return sh.Favicon
	}
	return icon.KeyOutlinedIconThemed
}

func (sh *SSHKey) Edit(ctx context.Context, key *paw.Key, w fyne.Window) (fyne.CanvasObject, paw.Item) {
	sshKeyItem := &paw.SSHKey{}
	*sshKeyItem = *sh.SSHKey
	sshKeyItem.Metadata = &paw.Metadata{}
	*sshKeyItem.Metadata = *sh.Metadata
	sshKeyItem.Note = &paw.Note{}
	*sshKeyItem.Note = *sh.Note

	titleEntryBind := binding.BindString(&sshKeyItem.Name)
	titleEntry := widget.NewEntryWithData(titleEntryBind)
	titleEntry.Validator = nil
	titleEntry.PlaceHolder = "Untitled SSH Key"

	publicKeyEntryBind := binding.BindString(&sshKeyItem.PublicKey)
	publicKeyEntry := widget.NewEntryWithData(publicKeyEntryBind)
	publicKeyEntry.Validator = nil
	publicKeyEntry.MultiLine = true
	publicKeyEntry.Wrapping = fyne.TextWrapBreak
	publicKeyEntry.Disable()
	publicKeyCopyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(publicKeyEntry.Text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: "Public Key copied to clipboard",
		})
	})
	publicKeyExportButton := widget.NewButtonWithIcon("Export", icon.DownloadOutlinedIconThemed, func() {
		d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if uc == nil {
				// file open dialog has been cancelled
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
	})

	fingerprintEntryBind := binding.BindString(&sshKeyItem.Fingerprint)
	fingerprintEntry := widget.NewLabelWithData(fingerprintEntryBind)

	privateKeyEntryBind := binding.BindString(&sshKeyItem.PrivateKey)
	privateKeyEntry := widget.NewEntryWithData(privateKeyEntryBind)
	privateKeyEntry.Validator = nil
	privateKeyEntry.MultiLine = true
	privateKeyEntry.Disable()
	privateKeyEntry.SetPlaceHolder("Private Key")
	privateKeyCopyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(privateKeyEntry.Text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: "Private Key copied to clipboard",
		})
	})
	privateKeyMakeButton := widget.NewButtonWithIcon("Generate", icon.KeyOutlinedIconThemed, func() {
		sk, err := sshkey.GenerateKey()
		if err != nil {
			dialog.NewError(err, w).Show()
			return
		}
		privateKeyEntryBind.Set(string(sk.PrivateKey()))
		publicKeyEntryBind.Set(string(sk.PublicKey()))
		fingerprintEntryBind.Set(string(sk.Fingerprint()))
	})

	privateKeyImportButton := widget.NewButtonWithIcon("Import", icon.UploadOutlinedIconThemed, func() {
		d := dialog.NewFileOpen(func(uc fyne.URIReadCloser, e error) {
			b, err := io.ReadAll(uc)
			uc.Close()
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			sk, err := sshkey.ParseKey(b)
			if err != nil {
				dialog.NewError(err, w).Show()
				return
			}
			privateKeyEntryBind.Set(string(sk.PrivateKey()))
			publicKeyEntryBind.Set(string(sk.PublicKey()))
			fingerprintEntryBind.Set(string(sk.Fingerprint()))
		}, w)
		d.Show()
	})

	privateKeyExportButton := widget.NewButtonWithIcon("Export", icon.DownloadOutlinedIconThemed, func() {
		d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if uc == nil {
				// file open dialog has been cancelled
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
	})

	noteEntry := widget.NewEntryWithData(binding.BindString(&sshKeyItem.Note.Value))
	noteEntry.MultiLine = true
	noteEntry.Validator = nil

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(sh.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Private Key"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(privateKeyCopyButton, privateKeyMakeButton, privateKeyImportButton, privateKeyExportButton), privateKeyEntry))

	form.Add(labelWithStyle("Public Key"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(publicKeyCopyButton, publicKeyExportButton), publicKeyEntry))

	form.Add(labelWithStyle("Fingerprint"))
	form.Add(fingerprintEntry)

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, sshKeyItem
}

func (sh *SSHKey) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(sh.Icon(), sh.Name)
	obj = append(obj, rowWithAction("Private Key", sh.PrivateKey, rowActions{copy: true, ellipsis: 64, export: sh.Name}, w)...)
	obj = append(obj, rowWithAction("Public Key", sh.PrivateKey, rowActions{copy: true, ellipsis: 64, export: sh.Name + ".pub"}, w)...)
	obj = append(obj, copiableRow("Fingerprint", sh.Fingerprint, w)...)
	obj = append(obj, copiableRow("Note", sh.Note.Value, w)...)
	return container.New(layout.NewFormLayout(), obj...)
}
