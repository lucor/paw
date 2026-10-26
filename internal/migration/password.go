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

// MigratePassword converts a legacy password to a CXF-backed Paw item.
func MigratePassword(password *paw.Password) (*model.Item, error) {
	if password == nil || password.Metadata == nil || password.Type != paw.PasswordItemType {
		return nil, fmt.Errorf("cannot migrate password: invalid metadata")
	}
	if !utf8.ValidString(password.Subtitle) {
		return nil, fmt.Errorf("cannot migrate password: invalid UTF-8")
	}
	created, err := timestamp(password.Created)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate password creation date: %w", err)
	}
	modified, err := timestamp(password.Modified)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate password modification date: %w", err)
	}
	item, err := model.NewPassword(password.Name, password.Value, model.PasswordSettings{
		Mode:   password.Mode,
		Length: password.Length,
		Format: password.Format,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot migrate password: %w", err)
	}
	item.CreationAt = created
	item.ModifiedAt = modified
	if password.Subtitle != "" {
		subtitle := password.Subtitle
		item.Subtitle = &subtitle
	}
	if password.Note != nil && password.Note.Value != "" {
		if !utf8.ValidString(password.Note.Value) {
			return nil, fmt.Errorf("cannot migrate password note: invalid UTF-8")
		}
		note, err := json.Marshal(cxf.Note{
			Type: cxf.CredentialTypeNote,
			Content: cxf.EditableField{
				FieldType: cxf.FieldTypeString,
				Value:     password.Note.Value,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("cannot encode password note")
		}
		item.Credentials = append(item.Credentials, note)
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot migrate password: %w", err)
	}
	return item, nil
}
