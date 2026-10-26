// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
)

// NewLogin creates a Paw login item with a new ID and current dates.
func NewLogin(title, username, password string, settings PasswordSettings) (*Item, error) {
	if !utf8.ValidString(title) || !utf8.ValidString(username) {
		return nil, fmt.Errorf("cannot create login: invalid UTF-8")
	}
	passwordField, err := newPasswordField(password, settings)
	if err != nil {
		return nil, fmt.Errorf("cannot create login password: %w", err)
	}
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate login ID: %w", err)
	}
	credential, err := json.Marshal(cxf.BasicAuth{
		Type: cxf.CredentialTypeBasicAuth,
		Username: &cxf.EditableField{
			FieldType: cxf.FieldTypeString,
			Value:     username,
		},
		Password: passwordField,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode login credential")
	}
	created := uint64(time.Now().Unix())
	modified := created
	metadata := ItemMetadata{Type: paw.LoginItemType}
	extension, err := encodeItemExtension(nil, metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot encode login metadata: %w", err)
	}
	item := &Item{
		Item: cxf.Item{
			ID:          id,
			Title:       title,
			CreationAt:  &created,
			ModifiedAt:  &modified,
			Credentials: []json.RawMessage{credential},
			Extensions:  []json.RawMessage{extension},
		},
		Metadata: metadata,
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create login: %w", err)
	}
	return item, nil
}
