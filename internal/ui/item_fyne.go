// SPDX-FileCopyrightText: 2021-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/favicon"
	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/sshkey"
	pawwidget "go.lucor.dev/paw/internal/widget"
)

// FyneItemWidget displays and edits a CXF item.
type FyneItemWidget interface {
	Icon() fyne.Resource
	Show(ctx context.Context, w fyne.Window) fyne.CanvasObject
	Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject
	Item() *model.Item
	OnSubmit() (*model.Item, error)
	IsNew() bool
}

// FynePasswordGenerator shows Paw's password generator.
type FynePasswordGenerator interface {
	ShowPasswordGenerator(bind binding.String, password *paw.Password, w fyne.Window)
}

type modelItemWidget struct {
	item                                     *model.Item
	key                                      *paw.Key
	title, username, password, note, website *widget.Entry
	websiteEntry                             *loginURLEntry
	loginFavicon                             *widget.Icon
	addToAgent                               *widget.Check
	sshPublicKey                             *widget.Entry
	sshFingerprint                           *widget.Label
	sshPrivateKey                            *widget.Entry
	sshKeyType                               *widget.Label
	isNew                                    bool
	preferences                              *paw.Preferences
	passwordSpec                             *paw.Password
	totpSpec                                 *paw.TOTP
	sshSpec                                  *cxf.SSHKey
	ctx                                      context.Context
}

// NewFyneItemWidget creates a widget for a CXF-backed item.
func NewFyneItemWidget(item *model.Item, preferences *paw.Preferences) FyneItemWidget {
	return newModelItemWidget(item, preferences, false)
}

// newModelItemWidget creates an item widget with its required preferences.
func newModelItemWidget(item *model.Item, preferences *paw.Preferences, isNew bool) *modelItemWidget {
	if preferences == nil {
		preferences = paw.NewDefaultPreferences()
	}
	return &modelItemWidget{item: item, preferences: preferences, isNew: isNew}
}

// Item returns a copy of the edited CXF item.
func (iw *modelItemWidget) Item() *model.Item {
	data, err := json.Marshal(iw.item)
	if err != nil {
		panic(err)
	}
	var copy model.Item
	if err := json.Unmarshal(data, &copy); err != nil {
		panic(err)
	}
	return &copy
}

// IsNew reports whether the widget is creating an item.
func (iw *modelItemWidget) IsNew() bool { return iw.isNew }

// OnSubmit applies the widget fields and validates the resulting CXF item.
func (iw *modelItemWidget) OnSubmit() (*model.Item, error) {
	item := iw.Item()
	item.Title = iw.title.Text
	if item.Metadata.Type == paw.LoginItemType {
		if item.Scope == nil {
			item.Scope = &cxf.CredentialScope{URLs: []string{}, AndroidApps: []cxf.AndroidAppID{}}
		}
		loginURL := paw.NewLoginURL()
		if err := loginURL.Set(iw.website.Text); err != nil {
			return nil, err
		}
		if loginURL.URL().String() != "" && loginURL.URL().Scheme != "https" {
			return nil, fmt.Errorf("login URL must use HTTPS")
		}
		item.Scope.URLs = []string{loginURL.String()}
		credential, index, err := widgetBasicAuth(item)
		if err != nil {
			return nil, err
		}
		credential.Username = &cxf.EditableField{FieldType: cxf.FieldTypeString, Value: iw.username.Text}
		credential.Password = &cxf.EditableField{FieldType: cxf.FieldTypeConcealedString, Value: iw.password.Text}
		if err := putBasicAuth(item, index, credential); err != nil {
			return nil, err
		}
		if err := putNote(item, iw.note.Text); err != nil {
			return nil, err
		}
		if iw.totpSpec != nil && iw.totpSpec.Secret != "" {
			totp := cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: iw.totpSpec.Secret, Period: uint16(iw.totpSpec.Interval), Digits: uint16(iw.totpSpec.Digits), Algorithm: strings.ToLower(string(iw.totpSpec.Hash))}
			if err := putTOTP(item, totp); err != nil {
				return nil, err
			}
		} else {
			removeCredential(item, cxf.CredentialTypeTOTP)
		}
		if iw.username.Text != "" {
			subtitle := iw.username.Text
			item.Subtitle = &subtitle
		}
		if iw.passwordSpec != nil {
			iw.passwordSpec.Value = iw.password.Text
			credential, index, err := widgetBasicAuth(item)
			if err != nil {
				return nil, err
			}
			credential.Password, err = makeUIPasswordField(iw.password.Text, iw.passwordSpec)
			if err != nil {
				return nil, err
			}
			if err := putBasicAuth(item, index, credential); err != nil {
				return nil, err
			}
		}
		host := loginURL.URL().Hostname()
		if item.Metadata.Favicon != nil && item.Metadata.Favicon.Host != host {
			item.Metadata.Favicon = nil
		}
		if !iw.preferences.FaviconDownloader.Disabled && host != "" && item.Metadata.Favicon == nil {
			data, format, err := favicon.Download(iw.ctx, loginURL.URL(), favicon.Options{})
			if err == nil {
				item.Metadata.Favicon = paw.NewFavicon(host, data, format)
			}
		}
	} else if item.Metadata.Type == paw.PasswordItemType {
		credential, index, err := widgetBasicAuth(item)
		if err != nil {
			return nil, err
		}
		if iw.passwordSpec != nil {
			credential.Password, err = makeUIPasswordField(iw.password.Text, iw.passwordSpec)
			if err != nil {
				return nil, err
			}
		} else {
			credential.Password = &cxf.EditableField{FieldType: cxf.FieldTypeConcealedString, Value: iw.password.Text}
		}
		if err := putBasicAuth(item, index, credential); err != nil {
			return nil, err
		}
	} else if item.Metadata.Type == paw.NoteItemType {
		if err := putNote(item, iw.note.Text); err != nil {
			return nil, err
		}
	} else if item.Metadata.Type == paw.SSHKeyItemType && iw.addToAgent != nil {
		item.Metadata.AddToAgent = iw.addToAgent.Checked
		if iw.sshSpec != nil {
			data, err := json.Marshal(iw.sshSpec)
			if err != nil {
				return nil, err
			}
			replaceCredential(item, cxf.CredentialTypeSSHKey, data)
		}
		if iw.password != nil {
			credential, index, err := widgetBasicAuth(item)
			if err != nil {
				return nil, err
			}
			credential.Password, err = makeUIPasswordField(iw.password.Text, iw.passwordSpec)
			if err != nil {
				return nil, err
			}
			if err := putBasicAuth(item, index, credential); err != nil {
				return nil, err
			}
		}
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}
	return item, nil
}

// Icon returns the icon for the item's Paw category.
func (iw *modelItemWidget) Icon() fyne.Resource {
	if iw.item.Metadata.Type == paw.LoginItemType && iw.item.Metadata.Favicon != nil {
		return iw.item.Metadata.Favicon
	}
	switch iw.item.Metadata.Type {
	case paw.NoteItemType:
		return icon.NoteOutlinedIconThemed
	case paw.PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case paw.LoginItemType:
		return icon.WorldWWWOutlinedIconThemed
	case paw.SSHKeyItemType:
		return icon.KeyOutlinedIconThemed
	default:
		return icon.PawIcon
	}
}

// passwordEditor displays a password field with generation and copy actions.
func (iw *modelItemWidget) passwordEditor(w fyne.Window) fyne.CanvasObject {
	if iw.passwordSpec == nil {
		iw.passwordSpec = paw.NewCustomPassword()
	}
	generate := widget.NewButtonWithIcon("", icon.KeyOutlinedIconThemed, func() {
		bind := binding.NewString()
		bind.AddListener(binding.NewDataListener(func() { value, _ := bind.Get(); iw.password.SetText(value); iw.passwordSpec.Value = value }))
		NewPasswordGenerator(iw.key, iw.preferences.Password).ShowPasswordGenerator(bind, iw.passwordSpec, w)
	})
	copyButton := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() { fyne.CurrentApp().Clipboard().SetContent(iw.password.Text) })
	return container.NewBorder(nil, nil, nil, container.NewHBox(generate, copyButton), iw.password)
}

// uiPasswordSpec reads Paw's generator settings from a CXF password field.
func uiPasswordSpec(field *cxf.EditableField) *paw.Password {
	password := paw.NewCustomPassword()
	if field == nil {
		return password
	}
	password.Value = field.Value
	for _, raw := range field.Extensions {
		var extension struct {
			Name   string           `json:"name"`
			Mode   paw.PasswordMode `json:"mode"`
			Length int              `json:"length"`
			Format paw.Format       `json:"format"`
		}
		if json.Unmarshal(raw, &extension) == nil && extension.Name == "paw.pm/password" {
			password.Mode = extension.Mode
			password.Length = extension.Length
			password.Format = extension.Format
		}
	}
	return password
}

// makeUIPasswordField stores a password and its generator settings in CXF.
func makeUIPasswordField(value string, password *paw.Password) (*cxf.EditableField, error) {
	extension, err := json.Marshal(struct {
		Name   string           `json:"name"`
		Mode   paw.PasswordMode `json:"mode"`
		Length int              `json:"length"`
		Format paw.Format       `json:"format"`
	}{Name: "paw.pm/password", Mode: password.Mode, Length: password.Length, Format: password.Format})
	if err != nil {
		return nil, fmt.Errorf("cannot encode password settings")
	}
	return &cxf.EditableField{FieldType: cxf.FieldTypeConcealedString, Value: value, Extensions: []json.RawMessage{extension}}, nil
}

// widgetTOTP reads a CXF TOTP credential and its position from an item.
func widgetTOTP(item *model.Item) (*paw.TOTP, int, error) {
	for index, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return nil, -1, err
		}
		if typeName != cxf.CredentialTypeTOTP {
			continue
		}
		var credential cxf.TOTP
		if err := json.Unmarshal(raw, &credential); err != nil {
			return nil, -1, err
		}
		hash := paw.TOTPHash(strings.ToUpper(credential.Algorithm))
		return &paw.TOTP{Secret: credential.Secret, Digits: int(credential.Digits), Interval: int(credential.Period), Hash: hash}, index, nil
	}
	return nil, -1, nil
}

// putTOTP adds or replaces the TOTP credential in an item.
func putTOTP(item *model.Item, credential cxf.TOTP) error {
	data, err := json.Marshal(credential)
	if err != nil {
		return err
	}
	_, index, err := widgetTOTP(item)
	if err != nil {
		return err
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, data)
	} else {
		item.Credentials[index] = data
	}
	return nil
}

// removeCredential removes credentials with the given type from an item.
func removeCredential(item *model.Item, typeName string) {
	credentials := item.Credentials[:0]
	for _, raw := range item.Credentials {
		got, _ := cxf.CredentialTypeOf(raw)
		if got != typeName {
			credentials = append(credentials, raw)
		}
	}
	item.Credentials = credentials
}

// replaceCredential replaces the first credential of the given type or appends it.
func replaceCredential(item *model.Item, typeName string, data json.RawMessage) {
	for index, raw := range item.Credentials {
		got, _ := cxf.CredentialTypeOf(raw)
		if got == typeName {
			item.Credentials[index] = data
			return
		}
	}
	item.Credentials = append(item.Credentials, data)
}

// generateCXFSSHKey creates an SSH key credential with a PKCS #8 private key.
func generateCXFSSHKey() (cxf.SSHKey, error) {
	key, err := sshkey.GenerateKey()
	if err != nil {
		return cxf.SSHKey{}, err
	}
	return cxfSSHKeyFromPrivate(key.PrivateKey(), key.PublicKey().Type(), "")
}

// cxfSSHKeyFromPrivate encodes a private key as a CXF SSH credential.
func cxfSSHKeyFromPrivate(private crypto.PrivateKey, keyType, comment string) (cxf.SSHKey, error) {
	if key, ok := private.(*ed25519.PrivateKey); ok {
		private = *key
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return cxf.SSHKey{}, fmt.Errorf("cannot encode SSH private key")
	}
	return cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: keyType, KeyComment: comment, PrivateKey: base64.RawURLEncoding.EncodeToString(der)}, nil
}

// displaySSHKey derives OpenSSH private and public text plus its fingerprint.
func displaySSHKey(credential cxf.SSHKey) (string, string, string, error) {
	der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH private key")
	}
	key, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return "", "", "", fmt.Errorf("invalid SSH private key")
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		return "", "", "", fmt.Errorf("unsupported SSH private key")
	}
	block, err := ssh.MarshalPrivateKey(key, credential.KeyComment)
	if err != nil {
		return "", "", "", err
	}
	return string(pem.EncodeToMemory(block)), string(ssh.MarshalAuthorizedKey(signer.PublicKey())), ssh.FingerprintSHA256(signer.PublicKey()), nil
}

// importSSHKey opens a key file and adds its key to the editor.
func (iw *modelItemWidget) importSSHKey(w fyne.Window) {
	dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		if reader == nil {
			return
		}
		defer reader.Close()
		data, err := io.ReadAll(reader)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		parsed, err := sshkey.ParseKey(data)
		if err != nil {
			if _, ok := err.(*ssh.PassphraseMissingError); !ok {
				dialog.ShowError(err, w)
				return
			}
			entry := widget.NewPasswordEntry()
			dialog.ShowForm("Private Key is password protected", "Import", "Cancel", []*widget.FormItem{widget.NewFormItem("Passphrase", entry)}, func(ok bool) {
				if !ok {
					return
				}
				parsed, err = sshkey.ParseKeyWithPassphrase(data, []byte(entry.Text))
				if err != nil {
					dialog.ShowError(err, w)
					return
				}
				iw.setImportedSSHKey(parsed.PrivateKey(), parsed.PublicKey().Type(), entry.Text, w)
			}, w)
			return
		}
		iw.setImportedSSHKey(parsed.PrivateKey(), parsed.PublicKey().Type(), "", w)
	}, w).Show()
}

// setImportedSSHKey stores an imported key and updates its public details.
func (iw *modelItemWidget) setImportedSSHKey(private crypto.PrivateKey, keyType, passphrase string, w fyne.Window) {
	credential, err := cxfSSHKeyFromPrivate(private, keyType, iw.title.Text)
	if err != nil {
		dialog.ShowError(err, w)
		return
	}
	if err := iw.setSSHCredential(credential, passphrase); err != nil {
		dialog.ShowError(err, w)
	}
}

// setSSHCredential updates the key and passphrase shown in the editor.
func (iw *modelItemWidget) setSSHCredential(credential cxf.SSHKey, passphrase string) error {
	private, public, fingerprint, err := displaySSHKey(credential)
	if err != nil {
		return err
	}
	iw.sshSpec = &credential
	iw.sshPublicKey.SetText(public)
	iw.sshFingerprint.SetText(fingerprint)
	iw.sshPrivateKey.SetText(private)
	iw.sshKeyType.SetText(credential.KeyType)
	if iw.password != nil {
		iw.password.SetText(passphrase)
		if iw.passwordSpec != nil {
			iw.passwordSpec.Value = passphrase
		}
	}
	return nil
}

// Show displays the item's CXF credential fields.
func (iw *modelItemWidget) Show(ctx context.Context, w fyne.Window) fyne.CanvasObject {
	item := iw.item
	objects := titleRow(iw.Icon(), item.Title)
	switch item.Metadata.Type {
	case paw.NoteItemType:
		if note, _, err := widgetNote(item); err == nil {
			objects = append(objects, rowWithAction("Note", note.Content.Value, rowActionOptions{copy: true}, w)...)
		}
	case paw.SSHKeyItemType:
		if credential, err := uiSSHCredential(item); err == nil {
			private, public, fingerprint, keyErr := displaySSHKey(credential)
			if keyErr == nil {
				return container.NewVScroll(container.NewVBox(
					container.NewHBox(objects...),
					sshKeyDetails(sshKeyEntry(public), sshKeyEntry(private), widget.NewLabel(credential.KeyType), widget.NewLabel(fingerprint), item.Title, w),
				))
			}
		}
	case paw.LoginItemType:
		if item.Scope != nil && len(item.Scope.URLs) > 0 {
			objects = append(objects, rowWithAction("URL", item.Scope.URLs[0], rowActionOptions{widgetType: "url", copy: true}, w)...)
		}
		if credential, _, err := widgetBasicAuth(item); err == nil {
			objects = append(objects, rowWithAction("Username", fieldValue(credential.Username), rowActionOptions{copy: true}, w)...)
			objects = append(objects, rowWithAction("Password", fieldValue(credential.Password), rowActionOptions{widgetType: "password", copy: true}, w)...)
		}
		if totp, _, err := widgetTOTP(item); err == nil && totp != nil && totp.Secret != "" {
			objects = append(objects, (&TOTP{TOTP: totp}).Show(ctx, w)...)
		}
		if note, _, err := widgetNote(item); err == nil && note.Content.Value != "" {
			objects = append(objects, rowWithAction("Note", note.Content.Value, rowActionOptions{copy: true}, w)...)
		}
	case paw.PasswordItemType:
		if credential, _, err := widgetBasicAuth(item); err == nil {
			objects = append(objects, rowWithAction("Password", fieldValue(credential.Password), rowActionOptions{widgetType: "password", copy: true}, w)...)
		}
	}
	return container.New(layout.NewFormLayout(), objects...)
}

// Edit builds fields for changing the item's CXF credentials.
func (iw *modelItemWidget) Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject {
	iw.key = key
	if ctx == nil {
		ctx = context.Background()
	}
	iw.ctx = ctx
	form := container.New(layout.NewFormLayout())
	iw.title = widget.NewEntry()
	iw.title.SetText(iw.item.Title)
	iw.title.Validator = requiredValidator("The title cannot be empty")
	if iw.item.Metadata.Type == paw.LoginItemType {
		iw.loginFavicon = widget.NewIcon(iw.Icon())
		form.Add(container.NewCenter(iw.loginFavicon))
	} else {
		form.Add(labelWithStyle("Title"))
	}
	form.Add(iw.title)
	switch iw.item.Metadata.Type {
	case paw.NoteItemType:
		note, _, _ := widgetNote(iw.item)
		iw.note = widget.NewMultiLineEntry()
		iw.note.SetText(note.Content.Value)
		form.Add(labelWithStyle("Note"))
		form.Add(iw.note)
	case paw.LoginItemType:
		urlText := ""
		if iw.item.Scope != nil && len(iw.item.Scope.URLs) > 0 {
			urlText = iw.item.Scope.URLs[0]
		}
		iw.websiteEntry = newLoginURLEntry(ctx, urlText, iw.preferences.FaviconDownloader)
		iw.websiteEntry.FaviconListener = func(favicon *paw.Favicon) {
			iw.item.Metadata.Favicon = favicon
			iw.loginFavicon.SetResource(iw.Icon())
		}
		iw.website = &iw.websiteEntry.Entry
		credential, _, _ := widgetBasicAuth(iw.item)
		iw.username = widget.NewEntry()
		iw.username.SetText(fieldValue(credential.Username))
		iw.password = widget.NewPasswordEntry()
		iw.password.SetText(fieldValue(credential.Password))
		iw.passwordSpec = uiPasswordSpec(credential.Password)
		form.Add(labelWithStyle("URL"))
		form.Add(iw.websiteEntry)
		form.Add(labelWithStyle("Username"))
		form.Add(iw.username)
		form.Add(labelWithStyle("Password"))
		form.Add(iw.passwordEditor(w))
		totp, _, _ := widgetTOTP(iw.item)
		iw.totpSpec = totp
		if iw.totpSpec == nil {
			iw.totpSpec = paw.NewDefaultTOTP()
		}
		totpForm, updatedTOTP := (&TOTP{TOTP: iw.totpSpec}).Edit(ctx, w)
		iw.totpSpec = updatedTOTP
		form.Objects = append(form.Objects, totpForm.(*fyne.Container).Objects...)
		note, _, _ := widgetNote(iw.item)
		iw.note = widget.NewMultiLineEntry()
		iw.note.SetText(note.Content.Value)
		form.Add(labelWithStyle("Note"))
		form.Add(iw.note)
	case paw.PasswordItemType:
		credential, _, _ := widgetBasicAuth(iw.item)
		iw.password = widget.NewPasswordEntry()
		iw.password.SetText(fieldValue(credential.Password))
		iw.passwordSpec = uiPasswordSpec(credential.Password)
		form.Add(labelWithStyle("Password"))
		form.Add(iw.passwordEditor(w))
	case paw.SSHKeyItemType:
		iw.addToAgent = widget.NewCheck("Add to SSH agent", nil)
		iw.addToAgent.SetChecked(iw.item.Metadata.AddToAgent)
		credential, err := uiSSHCredential(iw.item)
		if err != nil {
			return form
		}
		iw.sshSpec = &credential
		privateText, publicText, fingerprint, err := displaySSHKey(credential)
		if err != nil {
			dialog.ShowError(err, w)
			return form
		}
		iw.sshPublicKey = sshKeyEntry(publicText)
		iw.sshPrivateKey = sshKeyEntry(privateText)
		iw.sshKeyType = widget.NewLabel(credential.KeyType)
		iw.sshFingerprint = widget.NewLabel(fingerprint)
		generateLabel := "Generate key"
		if iw.isNew {
			generateLabel = "Regenerate key"
		}
		generate := widget.NewButton(generateLabel, func() {
			value, err := generateCXFSSHKey()
			if err != nil {
				dialog.ShowError(err, w)
				return
			}
			value.KeyComment = iw.title.Text
			if err := iw.setSSHCredential(value, iw.password.Text); err != nil {
				dialog.ShowError(err, w)
			}
		})
		importKey := widget.NewButton("Import key", func() { iw.importSSHKey(w) })
		credentialPass, _, _ := widgetBasicAuth(iw.item)
		iw.password = widget.NewPasswordEntry()
		iw.password.SetText(fieldValue(credentialPass.Password))
		iw.passwordSpec = uiPasswordSpec(credentialPass.Password)
		form.Add(labelWithStyle("Key passphrase"))
		form.Add(iw.passwordEditor(w))
		replacementHint := widget.NewLabel("Generating or importing replaces the key when you save.")
		replacementHint.Wrapping = fyne.TextWrapWord
		return container.NewVScroll(container.NewVBox(
			form,
			iw.addToAgent,
			widget.NewSeparator(),
			replacementHint,
			container.NewGridWithColumns(2, generate, importKey),
			widget.NewSeparator(),
			sshKeyDetails(iw.sshPublicKey, iw.sshPrivateKey, iw.sshKeyType, iw.sshFingerprint, iw.item.Title, w),
		))
	}
	return form
}

// sshKeyEntry displays key material with native wrapping and scrolling.
func sshKeyEntry(text string) *widget.Entry {
	entry := widget.NewMultiLineEntry()
	entry.Wrapping = fyne.TextWrapBreak
	entry.SetText(text)
	entry.Disable()
	return entry
}

// sshKeyDetails shares the read-only key layout between viewing and editing.
func sshKeyDetails(public, private *widget.Entry, keyType, fingerprint *widget.Label, title string, w fyne.Window) fyne.CanvasObject {
	fingerprint.Wrapping = fyne.TextWrapBreak
	metadata := widget.NewForm(
		widget.NewFormItem("Key type", keyType),
		widget.NewFormItem("Fingerprint", container.NewBorder(nil, nil, nil,
			widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
				copyAction("Fingerprint", fingerprint.Text, w)()
			}), fingerprint)),
	)
	publicHeader := container.NewBorder(nil, nil, widget.NewLabel("Public key"), sshKeyActions("Public key", title+".pub", public, w))
	privateSection := widget.NewAccordion(widget.NewAccordionItem("Private key", container.NewVBox(
		sshKeyActions("Private key", title+".key", private, w),
		private,
	)))
	return container.NewVBox(metadata, publicHeader, public, privateSection)
}

// sshKeyActions reads the current field so replacement keys can be copied and downloaded.
func sshKeyActions(label, filename string, entry *widget.Entry, w fyne.Window) fyne.CanvasObject {
	copyButton := widget.NewButtonWithIcon("", theme.ContentCopyIcon(), func() {
		copyAction(label, entry.Text, w)()
	})
	downloadButton := widget.NewButtonWithIcon("", icon.DownloadOutlinedIconThemed, func() {
		exportAction(filename, []byte(entry.Text), w)()
	})
	return container.NewHBox(copyButton, downloadButton)
}

// widgetBasicAuth reads the first BasicAuth credential and its position.
func widgetBasicAuth(item *model.Item) (cxf.BasicAuth, int, error) {
	for i, raw := range item.Credentials {
		typeName, typeErr := cxf.CredentialTypeOf(raw)
		if typeErr != nil {
			return cxf.BasicAuth{}, -1, typeErr
		}
		if typeName == cxf.CredentialTypeBasicAuth {
			var value cxf.BasicAuth
			if err := json.Unmarshal(raw, &value); err != nil {
				return cxf.BasicAuth{}, -1, err
			}
			return value, i, nil
		}
	}
	return cxf.BasicAuth{Type: cxf.CredentialTypeBasicAuth}, -1, nil
}

// putBasicAuth adds or replaces a BasicAuth credential in an item.
func putBasicAuth(item *model.Item, index int, value cxf.BasicAuth) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, data)
	} else {
		item.Credentials[index] = data
	}
	return nil
}

// widgetNote reads the first Note credential and its position.
func widgetNote(item *model.Item) (cxf.Note, int, error) {
	for i, raw := range item.Credentials {
		typeName, typeErr := cxf.CredentialTypeOf(raw)
		if typeErr != nil {
			return cxf.Note{}, -1, typeErr
		}
		if typeName == cxf.CredentialTypeNote {
			var value cxf.Note
			if err := json.Unmarshal(raw, &value); err != nil {
				return cxf.Note{}, -1, err
			}
			return value, i, nil
		}
	}
	return cxf.Note{}, -1, nil
}

// putNote adds, updates, or removes the item's Note credential.
func putNote(item *model.Item, value string) error {
	note, index, err := widgetNote(item)
	if err != nil {
		return err
	}
	note.Type = cxf.CredentialTypeNote
	note.Content = cxf.EditableField{FieldType: cxf.FieldTypeString, Value: value}
	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	if index < 0 {
		item.Credentials = append(item.Credentials, data)
	} else {
		item.Credentials[index] = data
	}
	return nil
}

// fieldValue returns an editable field's text or an empty string.
func fieldValue(field *cxf.EditableField) string {
	if field == nil {
		return ""
	}
	return field.Value
}

// titleRow creates the heading row for an item view.
func titleRow(icon fyne.Resource, text string) []fyne.CanvasObject {
	t := canvas.NewText(text, theme.Color(theme.ColorNameForeground))
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextHeadingSize()
	i := canvas.NewImageFromResource(icon)
	i.FillMode = canvas.ImageFillContain
	i.SetMinSize(fyne.NewSize(32, 32))
	return []fyne.CanvasObject{container.NewCenter(i), t}
}

// labelWithStyle creates a form label using the app's standard style.
func labelWithStyle(label string) *widget.Label {
	return widget.NewLabelWithStyle(label, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
}

type rowActionOptions struct {
	widgetType string
	copy       bool
	export     string
}

// rowWithAction creates a labeled value row with its configured actions.
func rowWithAction(label, text string, opts rowActionOptions, w fyne.Window) []fyne.CanvasObject {
	var value fyne.CanvasObject
	switch opts.widgetType {
	case "password":
		value = pawwidget.NewPasswordRevealer(text)
	case "url":
		u, err := url.Parse(text)
		if err == nil && strings.HasPrefix(u.Scheme, "http") {
			value = widget.NewHyperlink(text, u)
		} else {
			value = widget.NewLabel(text)
		}
	default:
		value = widget.NewLabel(text)
	}
	buttons := []fyne.CanvasObject{}
	if opts.copy {
		buttons = append(buttons, widget.NewButtonWithIcon("", theme.ContentCopyIcon(), copyAction(label, text, w)))
	}
	if opts.export != "" {
		buttons = append(buttons, widget.NewButtonWithIcon("", icon.DownloadOutlinedIconThemed, exportAction(opts.export, []byte(text), w)))
	}
	var actions fyne.CanvasObject = widget.NewLabel("")
	if len(buttons) > 0 {
		actions = container.NewHBox(buttons...)
	}
	return []fyne.CanvasObject{labelWithStyle(label), container.NewBorder(nil, nil, nil, actions, value)}
}

// copyAction returns a callback that copies text and notifies the user.
func copyAction(label, text string, w fyne.Window) func() {
	return func() {
		fyne.CurrentApp().Clipboard().SetContent(text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{Title: "paw", Content: fmt.Sprintf("%s copied", label)})
	}
}

func exportAction(filename string, data []byte, w fyne.Window) func() {
	return func() {
		d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if uc != nil {
				defer uc.Close()
				_, _ = uc.Write(data)
			}
		}, w)
		d.SetFileName(filename)
		d.Show()
	}
}

// newValidatioError creates an error shown by an item field validator.
func newValidatioError(msg string) error { return &validationError{msg: msg} }

type validationError struct{ msg string }

// Error returns the validation message.
func (e *validationError) Error() string { return e.msg }
func requiredValidator(msg string) fyne.StringValidator {
	return func(text string) error {
		if text == "" {
			return newValidatioError(msg)
		}
		return nil
	}
}
