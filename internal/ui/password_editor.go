// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
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

// PasswordEditor provides the native form for creating or editing a Password item.
type PasswordEditor struct {
	View           fyne.CanvasObject
	BackButton     *iconButton
	SaveButton     *widget.Button
	GenerateButton *iconButton
	Title          *widget.Entry
	Password       *PasswordField
	PasswordSpec   *paw.Password

	item           *model.Item
	initialTitle   string
	initialSecret  string
	initialMode    paw.PasswordMode
	initialLength  int
	initialFormat  paw.Format
	basicAuth      *cxf.BasicAuth
	basicAuthIndex int
	titleError     *widget.Label
	headerTitle    *widget.Label
	form           *widget.Form
}

// NewPasswordEditor builds the create or edit screen for a Password item.
func NewPasswordEditor(item *model.Item, isNew bool) (*PasswordEditor, error) {
	if err := item.Validate(); err != nil {
		return nil, err
	}
	if item.Metadata.Type != paw.PasswordItemType {
		return nil, errors.New("item is not a password")
	}
	decoded, err := decodeEntryDetailCredentials(item.Credentials)
	if err != nil {
		return nil, err
	}
	if decoded.basicAuth == nil {
		return nil, errors.New("password credential is missing")
	}

	secret := ""
	if decoded.basicAuth.Password != nil {
		secret = decoded.basicAuth.Password.Value
	}
	editor := &PasswordEditor{
		item:           item,
		basicAuth:      decoded.basicAuth,
		basicAuthIndex: decoded.basicAuthIndex,
		Title:          widget.NewEntry(),
		Password:       NewPasswordField(),
		titleError:     newErrorText(""),
	}
	if !isNew {
		editor.Title.SetText(item.Title)
	}
	editor.Password.Entry.SetText(secret)
	editor.PasswordSpec = passwordSpecFromField(decoded.basicAuth.Password)
	editor.initialTitle = editor.Title.Text
	editor.initialSecret = secret
	editor.initialMode = editor.PasswordSpec.Mode
	editor.initialLength = editor.PasswordSpec.Length
	editor.initialFormat = editor.PasswordSpec.Format
	editor.titleError.Hide()
	editor.Title.OnChanged = func(value string) {
		updateTitleSaveButton(editor.SaveButton, value)
		if editor.titleError.Visible() && strings.TrimSpace(value) != "" {
			editor.titleError.Hide()
		}
	}
	editor.GenerateButton = newIconButton("Generate password", icons.Generate, nil)
	editor.Password.SetTrailingAction(editor.GenerateButton)
	editor.form = widget.NewForm(
		widget.NewFormItem("Title", container.NewVBox(editor.Title, editor.titleError)),
		widget.NewFormItem("Password", editor.Password),
	)
	editor.form.Orientation = widget.Vertical
	editor.BackButton = newIconButton("Back", icons.Back, nil)
	editor.BackButton.Importance = widget.LowImportance
	editor.SaveButton = widget.NewButton("Save", nil)
	editor.SaveButton.Importance = widget.HighImportance
	updateTitleSaveButton(editor.SaveButton, editor.Title.Text)
	title := "Edit password"
	if isNew {
		title = "New password"
	}
	editor.headerTitle = newText(title, titleStyle)
	header := newPageHeader(editor.BackButton, editor.headerTitle, editor.SaveButton)
	body := container.New(layout.NewCustomPaddedVBoxLayout(16), editor.form)
	editor.View = container.New(layout.NewCustomPaddedLayout(16, 24, 24, 24), container.NewBorder(header, nil, nil, nil, container.NewVScroll(body)))
	return editor, nil
}

// UpdatedItem validates the form and returns an updated copy of its Password item.
func (e *PasswordEditor) UpdatedItem() (*model.Item, error) {
	if strings.TrimSpace(e.Title.Text) == "" {
		err := errors.New("title is required")
		setFieldError(e.titleError, err)
		return nil, err
	}
	setFieldError(e.titleError, nil)
	updated := cloneItem(e.item)
	updated.Title = e.Title.Text
	credential := *e.basicAuth
	e.PasswordSpec.Value = e.Password.Entry.Text
	password, err := passwordFieldWithSettings(e.Password.Entry.Text, e.PasswordSpec, credential.Password)
	if err != nil {
		return nil, err
	}
	credential.Password = password
	if err := putBasicAuthCredential(updated, e.basicAuthIndex, credential); err != nil {
		return nil, err
	}
	if err := updated.Validate(); err != nil {
		return nil, err
	}
	return updated, nil
}

// IsDirty reports whether the user changed the title, password, or generator settings.
func (e *PasswordEditor) IsDirty() bool {
	return e.Title.Text != e.initialTitle ||
		e.Password.Entry.Text != e.initialSecret ||
		e.PasswordSpec.Mode != e.initialMode ||
		e.PasswordSpec.Length != e.initialLength ||
		e.PasswordSpec.Format != e.initialFormat
}

// HasValidationErrors reports whether the editor displays a field error.
func (e *PasswordEditor) HasValidationErrors() bool { return e.titleError.Visible() }
