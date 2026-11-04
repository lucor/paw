// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// LoginEditor provides a native form and an updated copy of its login item.
type LoginEditor struct {
	View           fyne.CanvasObject
	BackButton     *iconButton
	SaveButton     *widget.Button
	GenerateButton *iconButton
	Title          *widget.Entry
	Username       *widget.Entry
	Password       *PasswordField
	Website        *LoginURLEntry
	TOTPSecret     *widget.Entry
	TOTPSettings   *iconButton
	Notes          *widget.Entry
	PasswordSpec   *paw.PasswordSpec

	item           *model.Item
	initial        loginEditorValues
	basicAuth      *cxf.BasicAuth
	basicAuthIndex int
	note           *cxf.Note
	noteIndex      int
	totp           *cxf.TOTP
	totpIndex      int
	totpSettings   cxf.TOTP
	form           *widget.Form
	headerTitle    *widget.Label
	favicon        *iconPreview
	faviconValue   *model.Favicon
	titleError     *widget.Label
	websiteError   *widget.Label
	totpError      *widget.Label
}

// NewLoginEditor builds the create or edit screen for a Login item.
func NewLoginEditor(ctx context.Context, item *model.Item, faviconSettings model.FaviconDownloaderSettings, isNew bool) (*LoginEditor, error) {
	if err := item.Validate(); err != nil {
		return nil, err
	}
	if item.Metadata.Type != model.LoginItemType {
		return nil, fmt.Errorf("item is not a login")
	}
	decoded, err := model.DecodeCredentials(item.Credentials)
	if err != nil {
		return nil, err
	}
	if decoded.BasicAuth == nil {
		return nil, fmt.Errorf("login credential is missing")
	}

	editor := &LoginEditor{
		item:           item,
		basicAuth:      decoded.BasicAuth,
		basicAuthIndex: decoded.BasicAuthIndex,
		note:           decoded.Note,
		noteIndex:      decoded.NoteIndex,
		totp:           decoded.TOTP,
		totpIndex:      decoded.TOTPIndex,
		totpSettings: cxf.TOTP{
			Type: cxf.CredentialTypeTOTP, Algorithm: strings.ToLower(string(paw.TOTPHashDefault)),
			Period: uint16(paw.TOTPIntervalDefault), Digits: uint16(paw.TOTPDigitsDefault),
		},
		Title:        widget.NewEntry(),
		Username:     widget.NewEntry(),
		Password:     NewPasswordField(),
		Website:      NewLoginURLEntry(ctx, "", faviconSettings),
		TOTPSecret:   widget.NewPasswordEntry(),
		Notes:        widget.NewMultiLineEntry(),
		favicon:      newIconPreview(icons.Globe),
		titleError:   newErrorText(""),
		websiteError: newErrorText(""),
		totpError:    newErrorText(""),
	}
	username, password := "", ""
	if decoded.BasicAuth.Username != nil {
		username = decoded.BasicAuth.Username.Value
	}
	if decoded.BasicAuth.Password != nil {
		password = decoded.BasicAuth.Password.Value
	}
	website := ""
	if item.Scope != nil && len(item.Scope.URLs) > 0 {
		website = item.Scope.URLs[0]
	}
	notes := ""
	if decoded.Note != nil {
		notes = decoded.Note.Content.Value
	}
	if !isNew {
		editor.Title.SetText(item.Title)
	}
	editor.Username.SetText(username)
	editor.Password.Entry.SetText(password)
	if decoded.TOTP != nil {
		editor.TOTPSecret.SetText(decoded.TOTP.Secret)
		if !isNew {
			editor.totpSettings = *decoded.TOTP
		}
	}
	editor.Website.SetText(website)
	editor.Notes.SetText(notes)
	editor.PasswordSpec = passwordSpecFromField(decoded.BasicAuth.Password)
	editor.Website.PlaceHolder = "https://example.com"
	var previewFavicon *model.Favicon
	if item.Metadata.Favicon != nil && len(item.Metadata.Favicon.Data) > 0 {
		loginURL := paw.NewLoginURL()
		if loginURL.Set(website) == nil && item.Metadata.Favicon.Host == loginURL.URL().Hostname() {
			previewFavicon = item.Metadata.Favicon
		}
	}
	editor.faviconValue = previewFavicon
	editor.favicon.SetFavicon(previewFavicon)
	editor.Website.FaviconListener = func(favicon *model.Favicon) {
		editor.faviconValue = favicon
		editor.favicon.SetFavicon(favicon)
	}
	editor.titleError.Hide()
	editor.websiteError.Hide()
	editor.totpError.Hide()
	editor.initial = loginEditorValues{
		title: editor.Title.Text, username: username, password: password, website: website, notes: notes,
		totp:         editor.TOTPSecret.Text,
		totpSettings: editor.totpSettings,
		passwordMode: editor.PasswordSpec.Mode, passwordLength: editor.PasswordSpec.Length, passwordFormat: editor.PasswordSpec.Format,
	}
	editor.Title.OnChanged = func(value string) {
		updateTitleSaveButton(editor.SaveButton, value)
		if editor.titleError.Visible() && strings.TrimSpace(value) != "" {
			editor.titleError.Hide()
		}
	}
	editor.Website.OnChanged = func(value string) {
		if editor.websiteError.Visible() && validateLoginWebsite(value) == nil {
			editor.websiteError.Hide()
		}
	}
	editor.TOTPSettings = newIconButton("Authenticator settings", icons.Settings2, nil)
	editor.TOTPSecret.SetOnValidationChanged(func(err error) {
		editor.setTOTPError(err)
	})
	editor.TOTPSecret.Validator = editor.validateTOTPSecret
	editor.GenerateButton = newIconButton("Generate password", icons.Generate, nil)
	editor.Password.SetTrailingAction(editor.GenerateButton)

	titleRow := container.NewBorder(nil, nil, newEntryIconPreview(editor.favicon), nil, editor.Title)
	totpRow := container.NewBorder(nil, nil, nil, editor.TOTPSettings, editor.TOTPSecret)
	editor.form = widget.NewForm(
		widget.NewFormItem("Title", container.NewVBox(titleRow, editor.titleError)),
		widget.NewFormItem("Username", editor.Username),
		widget.NewFormItem("Password", editor.Password),
		widget.NewFormItem("Website", container.NewVBox(editor.Website, editor.websiteError)),
		widget.NewFormItem("Authenticator key", container.NewVBox(totpRow, editor.totpError)),
		widget.NewFormItem("Notes", editor.Notes),
	)
	editor.form.Orientation = widget.Vertical
	editor.BackButton = newIconButton("Back", icons.Back, nil)
	editor.BackButton.Importance = widget.LowImportance
	editor.SaveButton = widget.NewButton("Save", nil)
	editor.SaveButton.Importance = widget.HighImportance
	editor.Username.PlaceHolder = "name@example.com"
	updateTitleSaveButton(editor.SaveButton, editor.Title.Text)
	title := "Edit login"
	if isNew {
		title = "New login"
	}
	editor.headerTitle = newText(title, titleStyle)
	header := newPageHeader(editor.BackButton, editor.headerTitle, editor.SaveButton)
	body := container.New(layout.NewCustomPaddedVBoxLayout(pawSize(pawSizeNameFormSpacing)), editor.form)
	pagePadding := pawSize(pawSizeNamePagePadding)
	editor.View = container.New(layout.NewCustomPaddedLayout(pawSize(pawSizeNameFormSpacing), pagePadding, pagePadding, pagePadding), container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
	return editor, nil
}

// UpdatedItem validates the form and returns an updated copy of its login.
func (e *LoginEditor) UpdatedItem() (*model.Item, error) {
	if err := e.validateFields(); err != nil {
		return nil, err
	}
	loginURL := paw.NewLoginURL()
	if err := loginURL.Set(e.Website.Text); err != nil {
		return nil, errors.New("enter a valid website address")
	}
	if loginURL.String() != "" && loginURL.URL().Scheme != "https" {
		return nil, errors.New("website must use HTTPS")
	}

	updated := cloneItem(e.item)
	updated.Title = e.Title.Text
	updated.Scope = cloneCredentialScope(e.item.Scope)
	if updated.Scope == nil {
		updated.Scope = &cxf.CredentialScope{URLs: []string{}, AndroidApps: []cxf.AndroidAppID{}}
	}
	updated.Scope.URLs = []string{loginURL.String()}
	if e.Username.Text == "" {
		updated.Subtitle = nil
	} else {
		subtitle := e.Username.Text
		updated.Subtitle = &subtitle
	}
	updated.Metadata.Favicon = e.faviconValue
	if updated.Metadata.Favicon != nil && updated.Metadata.Favicon.Host != loginURL.URL().Hostname() {
		updated.Metadata.Favicon = nil
	}

	credential := *e.basicAuth
	credential.Username = &cxf.EditableField{FieldType: cxf.FieldTypeString, Value: e.Username.Text}
	e.PasswordSpec.Value = e.Password.Entry.Text
	password, err := passwordFieldWithSettings(e.Password.Entry.Text, e.PasswordSpec, credential.Password)
	if err != nil {
		return nil, err
	}
	credential.Password = password
	if err := updated.SetBasicAuth(e.basicAuthIndex, credential); err != nil {
		return nil, err
	}
	if e.note != nil || e.Notes.Text != "" {
		note := cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: e.Notes.Text}}
		if e.note != nil {
			note = *e.note
			note.Content.FieldType = cxf.FieldTypeString
			note.Content.Value = e.Notes.Text
		}
		if err := updated.SetNote(e.noteIndex, note); err != nil {
			return nil, err
		}
	}
	if err := e.putTOTP(updated); err != nil {
		return nil, err
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	return updated, nil
}

// IsDirty reports whether the user changed a Login field or password settings.
func (e *LoginEditor) IsDirty() bool {
	return e.Title.Text != e.initial.title ||
		e.Username.Text != e.initial.username ||
		e.Password.Entry.Text != e.initial.password ||
		e.Website.Text != e.initial.website ||
		e.TOTPSecret.Text != e.initial.totp ||
		e.totpSettings != e.initial.totpSettings ||
		e.Notes.Text != e.initial.notes ||
		e.PasswordSpec.Mode != e.initial.passwordMode ||
		e.PasswordSpec.Length != e.initial.passwordLength ||
		e.PasswordSpec.Format != e.initial.passwordFormat
}

// HasValidationErrors reports whether the editor displays a field error.
func (e *LoginEditor) HasValidationErrors() bool {
	return e.titleError.Visible() || e.websiteError.Visible() || e.totpError.Visible()
}

func (e *LoginEditor) validateFields() error {
	var titleErr error
	if strings.TrimSpace(e.Title.Text) == "" {
		titleErr = errors.New("title is required")
	}
	websiteErr := validateLoginWebsite(e.Website.Text)
	totpErr := e.validateTOTPSecret(e.TOTPSecret.Text)
	setFieldError(e.titleError, titleErr)
	setFieldError(e.websiteError, websiteErr)
	e.setTOTPError(totpErr)
	if titleErr != nil {
		return titleErr
	}
	if websiteErr != nil {
		return websiteErr
	}
	return totpErr
}

func (e *LoginEditor) setTOTPError(err error) {
	setFieldError(e.totpError, err)
	if e.form != nil {
		e.form.Refresh()
	}
}

func (e *LoginEditor) validateTOTPSecret(secret string) error {
	secret = normalizeTOTPSecret(secret)
	if secret == "" {
		return nil
	}
	credential := e.totpCredential(secret)
	if err := credential.Validate(); err != nil {
		return errors.New("enter a valid secret")
	}
	return nil
}

func normalizeTOTPSecret(secret string) string {
	return strings.ToUpper(strings.NewReplacer(" ", "", "\t", "", "\n", "", "\r", "", "-", "").Replace(secret))
}

func (e *LoginEditor) totpCredential(secret string) cxf.TOTP {
	credential := e.totpSettings
	credential.Secret = secret
	return credential
}

func (e *LoginEditor) putTOTP(item *model.Item) error {
	secret := normalizeTOTPSecret(e.TOTPSecret.Text)
	if secret == "" {
		if e.totpIndex < 0 {
			return nil
		}
		item.Credentials = append(item.Credentials[:e.totpIndex], item.Credentials[e.totpIndex+1:]...)
		return nil
	}
	if e.totp != nil && e.TOTPSecret.Text == e.initial.totp && e.totpSettings == e.initial.totpSettings {
		return nil
	}
	credential := e.totpCredential(secret)
	if err := credential.Validate(); err != nil {
		return errors.New("enter a valid secret")
	}
	encoded, err := json.Marshal(credential)
	if err != nil {
		return errors.New("cannot encode TOTP credential")
	}
	if e.totpIndex < 0 {
		item.Credentials = append(item.Credentials, encoded)
	} else {
		item.Credentials[e.totpIndex] = encoded
	}
	return nil
}

func setFieldError(label *widget.Label, err error) {
	if err == nil {
		label.SetText("")
		label.Hide()
		return
	}
	label.SetText(err.Error())
	label.Show()
}

func updateTitleSaveButton(button *widget.Button, title string) {
	if button == nil {
		return
	}
	if strings.TrimSpace(title) == "" {
		button.Disable()
		return
	}
	button.Enable()
}

type loginEditorValues struct {
	title, username, password, website, totp, notes string
	totpSettings                                    cxf.TOTP
	passwordMode                                    paw.PasswordMode
	passwordLength                                  int
	passwordFormat                                  paw.Format
}

// passwordSpecFromField adapts saved settings to the generator's temporary state.
func passwordSpecFromField(field *cxf.EditableField) *paw.PasswordSpec {
	settings := model.PasswordSettingsFromField(field)
	password := &paw.PasswordSpec{Mode: settings.Mode, Length: settings.Length, Format: settings.Format}
	if field != nil {
		password.Value = field.Value
	}
	return password
}

func passwordFieldWithSettings(value string, settings *paw.PasswordSpec, previous *cxf.EditableField) (*cxf.EditableField, error) {
	return model.PasswordField(value, model.PasswordSettings{Mode: settings.Mode, Length: settings.Length, Format: settings.Format}, previous)
}

func cloneItem(item *model.Item) *model.Item {
	updated := *item
	updated.Credentials = make([]json.RawMessage, len(item.Credentials))
	for index, credential := range item.Credentials {
		updated.Credentials[index] = append(json.RawMessage(nil), credential...)
	}
	updated.Extensions = append([]json.RawMessage(nil), item.Extensions...)
	if item.CreationAt != nil {
		created := *item.CreationAt
		updated.CreationAt = &created
	}
	if item.ModifiedAt != nil {
		modified := *item.ModifiedAt
		updated.ModifiedAt = &modified
	}
	if item.Subtitle != nil {
		subtitle := *item.Subtitle
		updated.Subtitle = &subtitle
	}
	updated.Scope = cloneCredentialScope(item.Scope)
	return &updated
}

func cloneCredentialScope(scope *cxf.CredentialScope) *cxf.CredentialScope {
	if scope == nil {
		return nil
	}
	updated := *scope
	updated.URLs = append([]string{}, scope.URLs...)
	updated.AndroidApps = append([]cxf.AndroidAppID{}, scope.AndroidApps...)
	return &updated
}

func validateLoginWebsite(value string) error {
	if value == "" {
		return nil
	}
	loginURL := paw.NewLoginURL()
	if err := loginURL.Set(value); err != nil {
		return errors.New("enter a valid website address")
	}
	if loginURL.URL().Scheme != "https" {
		return errors.New("website must use HTTPS")
	}
	return nil
}
