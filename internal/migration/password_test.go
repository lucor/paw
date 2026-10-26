// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/migration"
	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// TestMigratePasswordPreservesData checks password fields and outer dates survive conversion.
func TestMigratePasswordPreservesData(t *testing.T) {
	created := time.Date(2020, 2, 3, 4, 5, 6, 123456789, time.FixedZone("test", 3600))
	modified := time.Date(2024, 7, 8, 9, 10, 11, 987654321, time.FixedZone("test", -25200))
	for _, mode := range []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			legacy := &paw.Password{
				Value:  "saved secret",
				Mode:   mode,
				Length: 24,
				Format: paw.LowercaseFormat | paw.DigitsFormat,
				Metadata: &paw.Metadata{
					Name: "Example password", Subtitle: "Account", Type: paw.PasswordItemType,
					Created: created, Modified: modified,
					Favicon:  &paw.Favicon{Host: "ignored.example"},
					Autofill: &paw.Autofill{TLDPlusOne: "ignored.example"},
				},
			}
			legacy.Note = &paw.Note{
				Value: "Recovery text",
				Metadata: &paw.Metadata{
					Name: "Ignored note title", Subtitle: "Ignored note subtitle", Type: paw.NoteItemType,
					Created:  time.Date(2001, 1, 2, 3, 4, 5, 0, time.UTC),
					Modified: time.Date(2002, 1, 2, 3, 4, 5, 0, time.UTC),
					Favicon:  &paw.Favicon{Host: "ignored-note.example"},
					Autofill: &paw.Autofill{TLDPlusOne: "ignored-note.example"},
				},
			}

			item, err := migration.MigratePassword(legacy)
			require.NoError(t, err)
			require.Equal(t, paw.PasswordItemType, item.Metadata.Type)
			require.Nil(t, item.Metadata.Favicon)
			require.Equal(t, legacy.Name, item.Title)
			require.Equal(t, legacy.Subtitle, *item.Subtitle)
			require.NotEqual(t, legacy.ID(), item.ID)
			require.Equal(t, uint64(created.Unix()), *item.CreationAt)
			require.Equal(t, uint64(modified.Unix()), *item.ModifiedAt)
			require.Len(t, item.Credentials, 2)
			require.Len(t, item.Extensions, 1)

			var basicAuth cxf.BasicAuth
			require.NoError(t, json.Unmarshal(item.Credentials[0], &basicAuth))
			require.Equal(t, cxf.CredentialTypeBasicAuth, basicAuth.Type)
			require.Nil(t, basicAuth.Username)
			require.NotNil(t, basicAuth.Password)
			require.Equal(t, cxf.FieldTypeConcealedString, basicAuth.Password.FieldType)
			require.Equal(t, legacy.Value, basicAuth.Password.Value)
			var settings struct {
				Name   string           `json:"name"`
				Mode   paw.PasswordMode `json:"mode"`
				Length int              `json:"length"`
				Format paw.Format       `json:"format"`
			}
			require.Len(t, basicAuth.Password.Extensions, 1)
			require.NoError(t, json.Unmarshal(basicAuth.Password.Extensions[0], &settings))
			require.Equal(t, "paw.pm/password", settings.Name)
			require.Equal(t, legacy.Mode, settings.Mode)
			require.Equal(t, legacy.Length, settings.Length)
			require.Equal(t, legacy.Format, settings.Format)

			var note cxf.Note
			require.NoError(t, json.Unmarshal(item.Credentials[1], &note))
			require.Equal(t, cxf.CredentialTypeNote, note.Type)
			require.Equal(t, cxf.FieldTypeString, note.Content.FieldType)
			require.Equal(t, legacy.Note.Value, note.Content.Value)
			require.Empty(t, note.Content.Extensions)
			require.NotContains(t, string(item.Extensions[0]), "Ignored note")
			require.NotContains(t, string(item.Extensions[0]), "ignored-note.example")

			encoded, err := json.Marshal(item)
			require.NoError(t, err)
			var decoded model.Item
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, item.ID, decoded.ID)
			require.Equal(t, item.Credentials, decoded.Credentials)
		})
	}
}

// TestMigratePasswordNotePresence checks only non-empty attached notes are included.
func TestMigratePasswordNotePresence(t *testing.T) {
	for _, test := range []struct {
		name string
		note *paw.Note
		want int
	}{
		{name: "absent", want: 1},
		{name: "empty", note: &paw.Note{Value: ""}, want: 1},
		{name: "present", note: &paw.Note{Value: "Attached text"}, want: 2},
	} {
		t.Run(test.name, func(t *testing.T) {
			legacy := &paw.Password{Metadata: &paw.Metadata{Type: paw.PasswordItemType}, Note: test.note}
			item, err := migration.MigratePassword(legacy)
			require.NoError(t, err)
			require.Len(t, item.Credentials, test.want)
		})
	}
}

// TestMigratePasswordGeneratesNewID checks each conversion creates a fresh identity.
func TestMigratePasswordGeneratesNewID(t *testing.T) {
	legacy := &paw.Password{Metadata: &paw.Metadata{Type: paw.PasswordItemType, Name: "Legacy"}}
	first, err := migration.MigratePassword(legacy)
	require.NoError(t, err)
	second, err := migration.MigratePassword(legacy)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
}

// TestMigratePasswordInvalidInput checks failures do not expose credential values.
func TestMigratePasswordInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		password *paw.Password
	}{
		{name: "nil password"},
		{name: "missing metadata", password: &paw.Password{Value: "private value"}},
		{name: "wrong type", password: &paw.Password{Value: "private value", Metadata: &paw.Metadata{Type: paw.LoginItemType}}},
		{name: "invalid title", password: &paw.Password{Value: "private value", Metadata: &paw.Metadata{Type: paw.PasswordItemType, Name: "bad\xff"}}},
		{name: "invalid value", password: &paw.Password{Value: "private\xff", Metadata: &paw.Metadata{Type: paw.PasswordItemType}}},
		{name: "invalid subtitle", password: &paw.Password{Metadata: &paw.Metadata{Type: paw.PasswordItemType, Subtitle: "bad\xff"}}},
		{name: "invalid note", password: &paw.Password{Metadata: &paw.Metadata{Type: paw.PasswordItemType}, Note: &paw.Note{Value: "private\xff"}}},
		{name: "unsupported mode", password: &paw.Password{Mode: paw.PasswordMode(4), Metadata: &paw.Metadata{Type: paw.PasswordItemType}}},
		{name: "unknown mode", password: &paw.Password{Mode: paw.PasswordMode(99), Metadata: &paw.Metadata{Type: paw.PasswordItemType}}},
		{name: "pre-epoch creation", password: &paw.Password{Metadata: &paw.Metadata{Type: paw.PasswordItemType, Created: time.Unix(-1, 0)}}},
		{name: "pre-epoch modification", password: &paw.Password{Metadata: &paw.Metadata{Type: paw.PasswordItemType, Modified: time.Unix(-1, 0)}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item, err := migration.MigratePassword(test.password)
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "private")
		})
	}
}
