// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
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
	}, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("cannot encode password credential")
	}
	created := uint64(time.Now().Unix())
	modified := created
	metadata := ItemMetadata{Type: PasswordItemType}
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
			Credentials: []jsontext.Value{credential},
			Extensions:  []jsontext.Value{itemExtension},
		},
		Metadata: metadata,
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create password: %w", err)
	}
	return item, nil
}

// newPasswordField creates a concealed field using the shared update rules.
func newPasswordField(value string, settings PasswordSettings) (*cxf.EditableField, error) {
	return PasswordField(value, settings, nil)
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
