package migration_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/migration"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestMigrateNote checks note content is preserved and dates become Unix seconds.
func TestMigrateNote(t *testing.T) {
	autofillURL, err := url.Parse("https://example.com/login?next=a%2Fb")
	require.NoError(t, err)
	tests := []struct {
		name string
		note *paw.Note
	}{
		{
			name: "all metadata",
			note: &paw.Note{
				Value: "First line\nUnicode: café 🔐\n",
				Metadata: &paw.Metadata{
					Name:     "Example note",
					Subtitle: "Example subtitle",
					Type:     paw.NoteItemType,
					Created:  time.Date(2020, 2, 3, 4, 5, 6, 123456789, time.FixedZone("", 19800)),
					Modified: time.Date(2024, 7, 8, 9, 10, 11, 987654321, time.FixedZone("", -25200)),
					Favicon: &paw.Favicon{
						Host: "example.com", Data: []byte{0, 1, 2, 255}, Format: "png",
					},
					Autofill: &paw.Autofill{
						AllowHTTP: true, TLDPlusOne: "example.com", URL: autofillURL,
					},
				},
			},
		},
		{
			name: "empty note and unset dates",
			note: &paw.Note{Metadata: &paw.Metadata{Type: paw.NoteItemType}},
		},
		{
			name: "epoch timestamp",
			note: &paw.Note{Metadata: &paw.Metadata{Type: paw.NoteItemType, Created: time.Unix(0, 0).UTC()}},
		},
		{
			name: "future timestamp",
			note: &paw.Note{Metadata: &paw.Metadata{Type: paw.NoteItemType, Created: time.Date(10000, 1, 2, 3, 4, 5, 6, time.UTC)}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := migration.MigrateNote(tt.note)
			require.NoError(t, err)
			require.Equal(t, paw.NoteItemType, item.Metadata.Type)
			require.Nil(t, item.Metadata.Favicon)
			id, err := base64.RawURLEncoding.Strict().DecodeString(item.ID)
			require.NoError(t, err)
			require.Len(t, id, 16)
			require.Equal(t, tt.note.Name, item.Title)
			if tt.note.Subtitle == "" {
				require.Nil(t, item.Subtitle)
			} else {
				require.NotNil(t, item.Subtitle)
				require.Equal(t, tt.note.Subtitle, *item.Subtitle)
			}
			require.Nil(t, item.Scope)
			if tt.note.Created.IsZero() {
				require.Nil(t, item.CreationAt)
			} else {
				require.NotNil(t, item.CreationAt)
				require.Equal(t, uint64(tt.note.Created.Unix()), *item.CreationAt)
				require.True(t, tt.note.Created.Truncate(time.Second).Equal(time.Unix(int64(*item.CreationAt), 0).UTC()))
			}
			if tt.note.Modified.IsZero() {
				require.Nil(t, item.ModifiedAt)
			} else {
				require.NotNil(t, item.ModifiedAt)
				require.Equal(t, uint64(tt.note.Modified.Unix()), *item.ModifiedAt)
			}
			require.Len(t, item.Credentials, 1)
			var credential cxf.Note
			require.NoError(t, json.Unmarshal(item.Credentials[0], &credential))
			require.Equal(t, cxf.CredentialTypeNote, credential.Type)
			require.Equal(t, cxf.FieldTypeString, credential.Content.FieldType)
			require.Equal(t, tt.note.Value, credential.Content.Value)

			// The CXF document API confirms the migrated item satisfies the schema.
			header := &cxf.Header{
				Version: cxf.Version{Major: 1}, ExporterRPID: "paw.pm", ExporterDisplayName: "Paw",
				Accounts: []cxf.Account{{ID: "AQ", Collections: []cxf.Collection{}, Items: []cxf.Item{item.Item}}},
			}
			var encoded bytes.Buffer
			require.NoError(t, cxf.Encode(&encoded, header))
			decoded, err := cxf.Decode(&encoded)
			require.NoError(t, err)
			migrated := decoded.Accounts[0].Items[0]
			require.Equal(t, item.ID, migrated.ID)
			var migratedNote cxf.Note
			require.NoError(t, json.Unmarshal(migrated.Credentials[0], &migratedNote))
			require.Equal(t, credential, migratedNote)

			var extension struct {
				Name     string          `json:"name"`
				Type     paw.ItemType    `json:"type"`
				Created  json.RawMessage `json:"created"`
				Modified json.RawMessage `json:"modified"`
				Favicon  *paw.Favicon    `json:"favicon,omitempty"`
				Autofill *paw.Autofill   `json:"autofill,omitempty"`
			}
			require.Len(t, migrated.Extensions, 1)
			require.NoError(t, json.Unmarshal(migrated.Extensions[0], &extension))
			require.Equal(t, "paw.pm/item", extension.Name)
			require.Equal(t, paw.NoteItemType, extension.Type)
			require.Empty(t, extension.Created)
			require.Empty(t, extension.Modified)
			require.Nil(t, extension.Favicon)
			require.Nil(t, extension.Autofill)
		})
	}
}

// TestNewAndMigratedNotes checks both paths can use the same native CXF workflow.
func TestNewAndMigratedNotes(t *testing.T) {
	newNote, err := model.NewNote("New note", "New content")
	require.NoError(t, err)

	legacy := paw.NewNote()
	legacy.Name = "Legacy note"
	legacy.Value = "Legacy content"
	migrated, err := migration.MigrateNote(legacy)
	require.NoError(t, err)

	items := []cxf.Item{newNote.Item, migrated.Item}
	for i := range items {
		item := &items[i]
		originalID := item.ID
		originalExtensions := append([]json.RawMessage(nil), item.Extensions...)

		// Both paths edit the standard credential directly, without a Paw Note wrapper.
		var note cxf.Note
		require.NoError(t, json.Unmarshal(item.Credentials[0], &note))
		note.Content.Value += " (edited)"
		item.Title += " (renamed)"
		item.Credentials[0], err = json.Marshal(note)
		require.NoError(t, err)
		require.NoError(t, item.Validate())
		require.Equal(t, originalID, item.ID)
		require.Equal(t, originalExtensions, item.Extensions)
	}

	var encoded bytes.Buffer
	require.NoError(t, cxf.Encode(&encoded, &cxf.Header{
		Version: cxf.Version{Major: 1}, ExporterRPID: "paw.pm", ExporterDisplayName: "Paw",
		Accounts: []cxf.Account{{ID: "AQ", Collections: []cxf.Collection{}, Items: items}},
	}))
	decoded, err := cxf.Decode(&encoded)
	require.NoError(t, err)
	require.Len(t, decoded.Accounts[0].Items, 2)
	for i, want := range []string{"New content (edited)", "Legacy content (edited)"} {
		item := decoded.Accounts[0].Items[i]
		require.Equal(t, items[i].ID, item.ID)
		require.Equal(t, items[i].Title, item.Title)
		require.Len(t, item.Extensions, len(items[i].Extensions))
		var note cxf.Note
		require.NoError(t, json.Unmarshal(item.Credentials[0], &note))
		require.Equal(t, want, note.Content.Value)
	}
}

// TestMigrateNoteGeneratesNewID checks migrations create new item identities.
func TestMigrateNoteGeneratesNewID(t *testing.T) {
	note := paw.NewNote()
	note.Name = "Original name"
	first, err := migration.MigrateNote(note)
	require.NoError(t, err)
	second, err := migration.MigrateNote(note)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	note.Name = "Renamed note"
	updated, err := migration.MigrateNote(note)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, updated.ID)
	require.Equal(t, "Renamed note", updated.Title)
}

// TestMigrateNoteInvalidInput checks failures without exposing note values.
func TestMigrateNoteInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		note *paw.Note
	}{
		{name: "nil note"},
		{name: "missing metadata", note: &paw.Note{Value: "private note"}},
		{name: "wrong type", note: &paw.Note{Value: "private note", Metadata: &paw.Metadata{Type: paw.LoginItemType}}},
		{name: "invalid text", note: &paw.Note{Value: "private note\xff", Metadata: &paw.Metadata{Type: paw.NoteItemType}}},
		{name: "invalid title", note: &paw.Note{Metadata: &paw.Metadata{Type: paw.NoteItemType, Name: "private note\xff"}}},
		{name: "pre-epoch creation", note: &paw.Note{Metadata: &paw.Metadata{Type: paw.NoteItemType, Created: time.Unix(-1, 0)}}},
		{name: "pre-epoch modification", note: &paw.Note{Metadata: &paw.Metadata{Type: paw.NoteItemType, Modified: time.Unix(-1, 0)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := migration.MigrateNote(tt.note)
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "private note")
		})
	}
}
