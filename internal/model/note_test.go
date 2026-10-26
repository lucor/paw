package model_test

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// TestNewNote checks new notes use valid CXF types and independent identities.
func TestNewNote(t *testing.T) {
	for _, tt := range []struct {
		name  string
		title string
		value string
	}{
		{name: "empty note"},
		{name: "text note", title: "Example note", value: "First line\nUnicode: café 🔐\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			before := uint64(time.Now().Unix())
			item, err := model.NewNote(tt.title, tt.value)
			require.NoError(t, err)
			require.NoError(t, item.Validate())
			require.Equal(t, paw.NoteItemType, item.Metadata.Type)
			require.Len(t, item.Extensions, 1)
			var metadata struct {
				Name string       `json:"name"`
				Type paw.ItemType `json:"type"`
			}
			require.NoError(t, json.Unmarshal(item.Extensions[0], &metadata))
			require.Equal(t, "paw.pm/item", metadata.Name)
			require.Equal(t, paw.NoteItemType, metadata.Type)
			require.Equal(t, tt.title, item.Title)
			id, err := base64.RawURLEncoding.Strict().DecodeString(item.ID)
			require.NoError(t, err)
			require.Len(t, id, 16)
			require.NotNil(t, item.CreationAt)
			require.NotNil(t, item.ModifiedAt)
			require.GreaterOrEqual(t, *item.CreationAt, before)
			require.LessOrEqual(t, *item.CreationAt, uint64(time.Now().Unix()))
			require.Equal(t, *item.CreationAt, *item.ModifiedAt)
			require.Len(t, item.Credentials, 1)
			var note cxf.Note
			require.NoError(t, json.Unmarshal(item.Credentials[0], &note))
			require.Equal(t, cxf.Note{
				Type:    cxf.CredentialTypeNote,
				Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: tt.value},
			}, note)

			other, err := model.NewNote(tt.title, tt.value)
			require.NoError(t, err)
			require.NotEqual(t, item.ID, other.ID)
			// Updating one date must not also update the other through a shared pointer.
			created := *item.CreationAt
			*item.ModifiedAt = created + 1
			require.Equal(t, created, *item.CreationAt)
		})
	}
}

// TestNewNoteInvalidText checks JSON encoding cannot silently replace invalid UTF-8.
func TestNewNoteInvalidText(t *testing.T) {
	for _, inputs := range [][2]string{{"private\xff", "value"}, {"title", "private\xff"}} {
		item, err := model.NewNote(inputs[0], inputs[1])
		require.Error(t, err)
		require.Nil(t, item)
		require.NotContains(t, err.Error(), "private")
	}
}
