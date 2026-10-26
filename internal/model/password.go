// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

const passwordExtensionName = "paw.pm/password"

// PasswordSettings stores the settings used to generate a password.
type PasswordSettings struct {
	Mode   paw.PasswordMode `json:"mode"`
	Length int              `json:"length"`
	Format paw.Format       `json:"format"`
}

// passwordExtension stores generation settings beside the password field.
type passwordExtension struct {
	Name string `json:"name"`
	PasswordSettings
}

// NewPassword creates a Paw password item with a new ID and current dates.
func NewPassword(title, value string, settings PasswordSettings) (*Item, error) {
	if !utf8.ValidString(title) {
		return nil, fmt.Errorf("cannot create password: invalid UTF-8")
	}
	password, err := newPasswordField(value, settings)
	if err != nil {
		return nil, fmt.Errorf("cannot create password: %w", err)
	}
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate password ID: %w", err)
	}
	credential, err := json.Marshal(cxf.BasicAuth{
		Type:     cxf.CredentialTypeBasicAuth,
		Password: password,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode password credential")
	}
	created := uint64(time.Now().Unix())
	modified := created
	metadata := ItemMetadata{Type: paw.PasswordItemType}
	itemExtension, err := encodeItemExtension(nil, metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot encode password item metadata: %w", err)
	}
	item := &Item{
		Item: cxf.Item{
			ID:          id,
			Title:       title,
			CreationAt:  &created,
			ModifiedAt:  &modified,
			Credentials: []json.RawMessage{credential},
			Extensions:  []json.RawMessage{itemExtension},
		},
		Metadata: metadata,
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create password: %w", err)
	}
	return item, nil
}

// newPasswordField creates a CXF password field with Paw's generation settings.
func newPasswordField(value string, settings PasswordSettings) (*cxf.EditableField, error) {
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("invalid UTF-8")
	}
	if !validPasswordMode(settings.Mode) {
		return nil, fmt.Errorf("unsupported mode %d", settings.Mode)
	}
	passwordSettings, err := json.Marshal(passwordExtension{
		Name:             passwordExtensionName,
		PasswordSettings: settings,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode password settings")
	}
	return &cxf.EditableField{
		FieldType:  cxf.FieldTypeConcealedString,
		Value:      value,
		Extensions: []json.RawMessage{passwordSettings},
	}, nil
}

// validPasswordMode reports whether Paw can represent the generation mode.
func validPasswordMode(mode paw.PasswordMode) bool {
	switch mode {
	case paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword:
		return true
	default:
		return false
	}
}
