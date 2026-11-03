// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// SSHKeyEditor provides the native form for creating or editing an SSH key.
type SSHKeyEditor struct {
	View                     fyne.CanvasObject
	BackButton               *iconButton
	SaveButton               *widget.Button
	GenerateButton           *widget.Button
	ImportButton             *widget.Button
	GeneratePassphraseButton *iconButton
	RevealPrivateButton      *iconButton
	RevealPassphraseButton   *iconButton
	CopyFingerprintButton    *iconButton
	CopyPublicButton         *iconButton
	ExportPublicButton       *iconButton
	CopyPrivateButton        *iconButton
	ExportPrivateButton      *iconButton
	Title                    *widget.Entry
	Passphrase               *widget.Entry
	AddToAgent               *widget.Check
	PasswordSpec             *paw.PasswordSpec

	item             *model.Item
	sshKey           *cxf.SSHKey
	basicAuth        *cxf.BasicAuth
	sshKeyIndex      int
	basicAuthIndex   int
	initialTitle     string
	initialPass      string
	initialAgent     bool
	initialKeyType   string
	initialPrivate   string
	publicText       string
	fingerprint      string
	fingerprintLabel *widget.Label
	keyType          *widget.Label
	publicPreview    *widget.Label
	privateBox       *fyne.Container
	privateLabel     *widget.Label
	titleError       *widget.Label
	headerTitle      *widget.Label
	form             *widget.Form
}

// NewSSHKeyEditor builds the create or edit screen for an SSH key item.
func NewSSHKeyEditor(item *model.Item, isNew bool) (*SSHKeyEditor, error) {
	if item == nil {
		return nil, errors.New("SSH key item is missing")
	}
	if err := item.Validate(); err != nil {
		return nil, err
	}
	if item.Metadata.Type != model.SSHKeyItemType {
		return nil, errors.New("item is not an SSH key")
	}
	item = cloneItem(item)
	decoded, err := decodeEntryDetailCredentials(item.Credentials)
	if err != nil {
		return nil, err
	}
	if decoded.sshKey == nil || decoded.basicAuth == nil {
		return nil, errors.New("SSH key credentials are incomplete")
	}
	_, public, fingerprint, err := FormatSSHKey(*decoded.sshKey)
	if err != nil {
		return nil, err
	}
	passphrase := ""
	if decoded.basicAuth.Password != nil {
		passphrase = decoded.basicAuth.Password.Value
	}
	editor := &SSHKeyEditor{
		item: item, sshKey: decoded.sshKey, basicAuth: decoded.basicAuth,
		sshKeyIndex: decoded.sshKeyIndex, basicAuthIndex: decoded.basicAuthIndex,
		Title: widget.NewEntry(), Passphrase: widget.NewPasswordEntry(),
		AddToAgent:   widget.NewCheck("Add to SSH agent", nil),
		PasswordSpec: passwordSpecFromField(decoded.basicAuth.Password),
		initialTitle: "", initialPass: passphrase, initialAgent: item.Metadata.AddToAgent,
		initialKeyType: decoded.sshKey.KeyType, initialPrivate: decoded.sshKey.PrivateKey,
		publicText: public, fingerprint: fingerprint,
		fingerprintLabel: newText(fingerprint, bodyStyle),
		keyType:          newText(strings.ToUpper(strings.TrimPrefix(decoded.sshKey.KeyType, "ssh-")), bodyStyle),
		publicPreview:    newText(compactSSHPreview(public), bodyStyle),
		privateBox:       container.NewVBox(), titleError: newErrorText(""),
	}
	if !isNew {
		editor.Title.SetText(item.Title)
		editor.initialTitle = item.Title
	}
	editor.Passphrase.SetText(passphrase)
	editor.AddToAgent.SetChecked(item.Metadata.AddToAgent)
	editor.keyType.Wrapping = fyne.TextWrapOff
	editor.keyType.Refresh()
	editor.publicPreview.Wrapping = fyne.TextWrapBreak
	editor.publicPreview.Refresh()
	editor.titleError.Hide()
	editor.Title.OnChanged = func(value string) {
		updateTitleSaveButton(editor.SaveButton, value)
		if editor.titleError.Visible() && strings.TrimSpace(value) != "" {
			editor.titleError.Hide()
		}
	}
	editor.RevealPassphraseButton = newIconButton("Show passphrase", icons.Show, func() { editor.togglePassphrase() })
	editor.GeneratePassphraseButton = newIconButton("Generate passphrase", icons.Generate, nil)
	passwordActions := container.NewHBox(
		editor.RevealPassphraseButton,
		editor.GeneratePassphraseButton,
		newIconButton("Copy passphrase", icons.Copy, func() { fyne.CurrentApp().Clipboard().SetContent(editor.Passphrase.Text) }),
	)
	passphraseRow := container.NewBorder(nil, nil, nil, passwordActions, editor.Passphrase)
	iconPreview := newEntryIcon(icons.SSH)
	titleRow := container.NewBorder(nil, nil, iconPreview, nil, editor.Title)
	keyHeading := container.NewBorder(nil, nil, newText("SSH key", captionStyle), editor.keyType, nil)
	replacementHint := newText("Generate or import replaces the key on save.", captionStyle)
	replacementHint.Wrapping = fyne.TextWrapOff
	replacementHint.Refresh()
	keyActions := container.NewGridWithColumns(2, editor.generateButton(), editor.importButton())
	copyFingerprint := newIconButton("Copy fingerprint", icons.Copy, nil)
	copyPublic := newIconButton("Copy public key", icons.Copy, nil)
	exportPublic := newIconButton("Export public key", icons.Download, nil)
	publicActions := container.NewHBox(copyPublic, exportPublic)
	publicHeader := container.NewBorder(nil, nil, newText("Public key", captionStyle), publicActions, widget.NewLabel(""))
	privateActions := container.NewHBox()
	privateRow := container.NewBorder(nil, nil, newText("Private key", captionStyle), privateActions, widget.NewLabel(""))
	help := newText("Stored securely in this vault", captionStyle)
	editor.form = widget.NewForm(
		widget.NewFormItem("Title", container.NewVBox(titleRow, editor.titleError)),
		widget.NewFormItem("Key passphrase", passphraseRow),
	)
	editor.form.Orientation = widget.Vertical
	editor.CopyFingerprintButton = copyFingerprint
	editor.CopyPublicButton = copyPublic
	editor.ExportPublicButton = exportPublic
	editor.RevealPrivateButton = newIconButton("Reveal private key", icons.Show, func() { editor.RevealPrivateKey() })
	editor.CopyPrivateButton = newIconButton("Copy private key", icons.Copy, nil)
	editor.ExportPrivateButton = newIconButton("Export private key", icons.Download, nil)
	privateActions.Add(editor.RevealPrivateButton)
	privateActions.Add(editor.CopyPrivateButton)
	privateActions.Add(editor.ExportPrivateButton)
	keySections := []fyne.CanvasObject{}
	keySummary := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)*2), keyHeading, replacementHint, keyActions)
	appendDetailSection(&keySections, keySummary)
	appendDetailSection(&keySections, newDetailField("Fingerprint", editor.fingerprintLabel, copyFingerprint))
	appendDetailSection(&keySections, container.NewVBox(publicHeader, editor.publicPreview))
	appendDetailSection(&keySections, container.NewVBox(privateRow, help, editor.privateBox))
	sectionObjects := append([]fyne.CanvasObject{widget.NewSeparator()}, keySections...)
	section := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)), sectionObjects...)
	editor.BackButton = newIconButton("Back", icons.Back, nil)
	editor.BackButton.Importance = widget.LowImportance
	editor.SaveButton = widget.NewButton("Save", nil)
	editor.SaveButton.Importance = widget.HighImportance
	updateTitleSaveButton(editor.SaveButton, editor.Title.Text)
	title := "Edit SSH key"
	if isNew {
		title = "New SSH key"
	}
	editor.headerTitle = newText(title, titleStyle)
	header := newPageHeader(editor.BackButton, editor.headerTitle, editor.SaveButton)
	body := container.New(layout.NewCustomPaddedVBoxLayout(16), editor.form, editor.AddToAgent, section)
	editor.View = container.New(layout.NewCustomPaddedLayout(16, 24, 24, 24), container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
	return editor, nil
}

// PublicKey returns the complete OpenSSH public key.
func (e *SSHKeyEditor) PublicKey() string { return e.publicText }

// Fingerprint returns the complete SSH fingerprint.
func (e *SSHKeyEditor) Fingerprint() string { return e.fingerprint }

func (e *SSHKeyEditor) generateButton() *widget.Button {
	e.GenerateButton = widget.NewButtonWithIcon("Generate", icons.GenerateKey, nil)
	e.GenerateButton.Importance = widget.MediumImportance
	return e.GenerateButton
}

func (e *SSHKeyEditor) importButton() *widget.Button {
	e.ImportButton = widget.NewButtonWithIcon("Import", icons.ImportKey, nil)
	e.ImportButton.Importance = widget.MediumImportance
	return e.ImportButton
}

func (e *SSHKeyEditor) togglePassphrase() {
	e.Passphrase.Password = !e.Passphrase.Password
	if e.Passphrase.Password {
		e.RevealPassphraseButton.Icon = icons.Show
		e.RevealPassphraseButton.label = "Show passphrase"
	} else {
		e.RevealPassphraseButton.Icon = icons.Hide
		e.RevealPassphraseButton.label = "Hide passphrase"
	}
	e.RevealPassphraseButton.Refresh()
	e.Passphrase.Refresh()
}

// SetCredential updates the displayed key and passphrase after generation or import.
func (e *SSHKeyEditor) SetCredential(credential cxf.SSHKey, passphrase string) error {
	_, public, fingerprint, err := FormatSSHKey(credential)
	if err != nil {
		return err
	}
	e.HidePrivateKey()
	e.sshKey = &credential
	e.publicText = public
	e.fingerprint = fingerprint
	e.fingerprintLabel.SetText(fingerprint)
	e.publicPreview.SetText(compactSSHPreview(public))
	e.keyType.SetText(strings.ToUpper(strings.TrimPrefix(credential.KeyType, "ssh-")))
	e.Passphrase.SetText(passphrase)
	e.PasswordSpec.Value = passphrase
	return nil
}

// RevealPrivateKey displays the complete key as read-only text.
func (e *SSHKeyEditor) RevealPrivateKey() error {
	if e.sshKey == nil {
		return errors.New("SSH key is no longer available")
	}
	private, _, _, err := FormatSSHKey(*e.sshKey)
	if err != nil {
		return err
	}
	e.privateLabel = newText(private, bodyStyle)
	e.privateLabel.TextStyle.Monospace = true
	e.privateLabel.Wrapping = fyne.TextWrapBreak
	e.privateLabel.Selectable = true
	e.privateLabel.Refresh()
	e.privateBox.Objects = []fyne.CanvasObject{e.privateLabel}
	e.privateBox.Refresh()
	e.RevealPrivateButton.Icon = icons.Hide
	e.RevealPrivateButton.label = "Hide private key"
	e.RevealPrivateButton.OnTapped = func() { e.HidePrivateKey() }
	return nil
}

// HidePrivateKey removes the private text from the editor UI.
func (e *SSHKeyEditor) HidePrivateKey() {
	if e.privateLabel != nil {
		e.privateLabel.SetText("")
		e.privateLabel = nil
	}
	e.privateBox.Objects = nil
	e.privateBox.Refresh()
	if e.RevealPrivateButton != nil {
		e.RevealPrivateButton.Icon = icons.Show
		e.RevealPrivateButton.label = "Reveal private key"
		e.RevealPrivateButton.OnTapped = func() {
			if err := e.RevealPrivateKey(); err != nil {
				return
			}
		}
		e.RevealPrivateButton.Refresh()
	}
}

// PrivateKeyText returns the complete OpenSSH private key for copy and export actions.
func (e *SSHKeyEditor) PrivateKeyText() (string, error) {
	if e.sshKey == nil {
		return "", errors.New("SSH key is no longer available")
	}
	private, _, _, err := FormatSSHKey(*e.sshKey)
	return private, err
}

// KeyChanged reports whether the key material or algorithm has changed.
func (e *SSHKeyEditor) KeyChanged() bool {
	if e.sshKey == nil {
		return false
	}
	return e.sshKey.KeyType != e.initialKeyType || e.sshKey.PrivateKey != e.initialPrivate
}

// IsDirty reports whether any editable SSH-key value changed.
func (e *SSHKeyEditor) IsDirty() bool {
	return e.Title.Text != e.initialTitle || e.Passphrase.Text != e.initialPass ||
		e.AddToAgent.Checked != e.initialAgent || e.KeyChanged()
}

// HasValidationErrors reports whether the editor displays a field error.
func (e *SSHKeyEditor) HasValidationErrors() bool { return e.titleError.Visible() }

// UpdatedItem validates the fields and returns an updated item copy.
func (e *SSHKeyEditor) UpdatedItem() (*model.Item, error) {
	if strings.TrimSpace(e.Title.Text) == "" {
		err := errors.New("title is required")
		setFieldError(e.titleError, err)
		return nil, err
	}
	setFieldError(e.titleError, nil)
	updated := cloneItem(e.item)
	updated.Title = e.Title.Text
	updated.Metadata.AddToAgent = e.AddToAgent.Checked
	keyData, err := json.Marshal(e.sshKey)
	if err != nil {
		return nil, fmt.Errorf("cannot encode SSH key: %w", err)
	}
	updated.Credentials[e.sshKeyIndex] = keyData
	credential := *e.basicAuth
	e.PasswordSpec.Value = e.Passphrase.Text
	credential.Password, err = passwordFieldWithSettings(e.Passphrase.Text, e.PasswordSpec, credential.Password)
	if err != nil {
		return nil, err
	}
	if err := putBasicAuthCredential(updated, e.basicAuthIndex, credential); err != nil {
		return nil, err
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	return updated, nil
}

// ClearSensitive removes transient key and passphrase values from editor widgets.
func (e *SSHKeyEditor) ClearSensitive() {
	e.HidePrivateKey()
	e.Passphrase.SetText("")
	e.PasswordSpec.Value = ""
	if e.item != nil {
		for index := range e.item.Credentials {
			for byteIndex := range e.item.Credentials[index] {
				e.item.Credentials[index][byteIndex] = 0
			}
			e.item.Credentials[index] = nil
		}
	}
	e.item = nil
	e.sshKey = nil
	e.basicAuth = nil
	e.initialPass = ""
	e.initialPrivate = ""
	e.publicText = ""
	e.fingerprint = ""
	e.publicPreview.SetText("")
	e.GenerateButton.OnTapped = nil
	e.ImportButton.OnTapped = nil
	e.GeneratePassphraseButton.OnTapped = nil
	e.RevealPrivateButton.OnTapped = nil
	e.RevealPassphraseButton.OnTapped = nil
	e.CopyFingerprintButton.OnTapped = nil
	e.CopyPublicButton.OnTapped = nil
	e.ExportPublicButton.OnTapped = nil
	e.CopyPrivateButton.OnTapped = nil
	e.ExportPrivateButton.OnTapped = nil
	e.SaveButton.OnTapped = nil
	e.BackButton.OnTapped = nil
}

func compactSSHPreview(public string) string {
	line := strings.TrimSpace(public)
	if len(line) > 64 {
		return line[:61] + "..."
	}
	return line
}
