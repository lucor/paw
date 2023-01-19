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

	publicKeyActionMenu := []*fyne.MenuItem{
		{
			Label: "Copy",
			Icon:  theme.ContentCopyIcon(),
			Action: func() {
				w.Clipboard().SetContent(publicKeyEntry.Text)
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

	fingerprintEntryBind := binding.BindString(&sshKeyItem.Fingerprint)
	fingerprintEntry := widget.NewLabelWithData(fingerprintEntryBind)
	fingerprintEntry.Wrapping = fyne.TextWrapBreak

	privateKeyEntryBind := binding.BindString(&sshKeyItem.PrivateKey)
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
				w.Clipboard().SetContent(privateKeyEntry.Text)
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
						dialog.NewError(err, w).Show()
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

	commentEntryBind := binding.BindString(&sshKeyItem.Comment)
	commentEntry := widget.NewEntryWithData(commentEntryBind)
	commentEntry.Validator = nil
	commentEntry.PlaceHolder = "Public Key Comment"

	addToAgentCheckBind := binding.BindBool(&sshKeyItem.AddToAgent)
	addToAgentCheck := widget.NewCheckWithData("", addToAgentCheckBind)

	noteEntry := newNoteEntryWithData(binding.BindString(&sshKeyItem.Note.Value))

	form := container.New(layout.NewFormLayout())
	form.Add(widget.NewIcon(sh.Icon()))
	form.Add(titleEntry)

	form.Add(labelWithStyle("Private Key"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(privateKeyActionMenu, w)), privateKeyEntry))

	form.Add(labelWithStyle("Public Key"))
	form.Add(container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(publicKeyActionMenu, w)), publicKeyEntry))

	form.Add(labelWithStyle("Comment"))
	form.Add(commentEntry)

	form.Add(labelWithStyle("Fingerprint"))
	form.Add(fingerprintEntry)

	form.Add(labelWithStyle("Add to SSH Agent"))
	form.Add(addToAgentCheck)

	form.Add(labelWithStyle("Note"))
	form.Add(noteEntry)

	return form, sshKeyItem
}

func (sh *SSHKey) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	obj := titleRow(sh.Icon(), sh.Name)
	obj = append(obj, rowWithAction("Private Key", sh.PrivateKey, rowActionOptions{copy: true, ellipsis: 64, export: sh.Name}, w)...)
	obj = append(obj, rowWithAction("Public Key", sh.PublicKey, rowActionOptions{copy: true, ellipsis: 64, export: sh.Name + ".pub"}, w)...)
	obj = append(obj, rowWithAction("Comment", sh.Comment, rowActionOptions{copy: true}, w)...)
	obj = append(obj, rowWithAction("Fingerprint", sh.Fingerprint, rowActionOptions{copy: true}, w)...)
	if sh.Note.Value != "" {
		obj = append(obj, rowWithAction("Note", sh.Note.Value, rowActionOptions{copy: true}, w)...)
	}
	v := "No"
	if sh.AddToAgent {
		v = "Yes"
	}
	obj = append(obj, rowWithAction("Add to SSH Agent", v, rowActionOptions{}, w)...)
	return container.New(layout.NewFormLayout(), obj...)
}
