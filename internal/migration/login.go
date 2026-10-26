// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// MigrateLogin converts a legacy login to a CXF-backed Paw item.
func MigrateLogin(login *paw.Login) (*model.Item, error) {
	if login == nil || login.Metadata == nil || login.Type != paw.LoginItemType {
		return nil, fmt.Errorf("cannot migrate login: invalid metadata")
	}
	if !utf8.ValidString(login.Metadata.Subtitle) {
		return nil, fmt.Errorf("cannot migrate login: invalid UTF-8")
	}
	created, err := timestamp(login.Created)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate login creation date: %w", err)
	}
	modified, err := timestamp(login.Modified)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate login modification date: %w", err)
	}
	settings := model.PasswordSettings{Mode: paw.CustomPassword}
	password := ""
	if login.Password != nil {
		password = login.Password.Value
		settings = model.PasswordSettings{
			Mode:   login.Password.Mode,
			Length: login.Password.Length,
			Format: login.Password.Format,
		}
	}
	item, err := model.NewLogin(login.Name, login.Username, password, settings)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate login: %w", err)
	}
	item.CreationAt = created
	item.ModifiedAt = modified
	if login.Metadata.Subtitle != "" {
		subtitle := login.Metadata.Subtitle
		item.Subtitle = &subtitle
	}
	if login.URL != nil && login.URL.String() != "" {
		item.Scope = &cxf.CredentialScope{
			URLs:        []string{login.URL.String()},
			AndroidApps: []cxf.AndroidAppID{},
		}
	}
	if login.Favicon != nil {
		favicon := *login.Favicon
		favicon.Data = append([]byte(nil), favicon.Data...)
		item.Metadata.Favicon = &favicon
	}
	if login.TOTP != nil {
		totp, err := MigrateTOTP(login.TOTP)
		if err != nil {
			return nil, fmt.Errorf("cannot migrate login TOTP: %w", err)
		}
		if totp != nil {
			credential, err := json.Marshal(totp)
			if err != nil {
				return nil, fmt.Errorf("cannot encode login TOTP")
			}
			item.Credentials = append(item.Credentials, credential)
		}
	}
	if login.Note != nil && login.Note.Value != "" {
		if !utf8.ValidString(login.Note.Value) {
			return nil, fmt.Errorf("cannot migrate login note: invalid UTF-8")
		}
		credential, err := json.Marshal(cxf.Note{
			Type: cxf.CredentialTypeNote,
			Content: cxf.EditableField{
				FieldType: cxf.FieldTypeString,
				Value:     login.Note.Value,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("cannot encode login note")
		}
		item.Credentials = append(item.Credentials, credential)
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot migrate login: %w", err)
	}
	return item, nil
}
