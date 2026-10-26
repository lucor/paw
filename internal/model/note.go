package model

import (
	"encoding/json"
	"fmt"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
)

// NewNote creates a Paw note item with a new ID and current dates.
func NewNote(title, content string) (*Item, error) {
	if !utf8.ValidString(title) || !utf8.ValidString(content) {
		return nil, fmt.Errorf("cannot create note: invalid UTF-8")
	}
	id, err := newID()
	if err != nil {
		return nil, fmt.Errorf("cannot generate note ID: %w", err)
	}
	credential, err := json.Marshal(cxf.Note{
		Type: cxf.CredentialTypeNote,
		Content: cxf.EditableField{
			FieldType: cxf.FieldTypeString,
			Value:     content,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode note credential")
	}
	created := uint64(time.Now().Unix())
	modified := created
	metadata := ItemMetadata{Type: paw.NoteItemType}
	extension, err := encodeItemExtension(nil, metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot encode note metadata: %w", err)
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
		return nil, fmt.Errorf("cannot create note: %w", err)
	}
	return item, nil
}
