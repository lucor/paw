package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"time"
	"unicode/utf8"

	"go.lucor.dev/cxf"
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
	}, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("cannot encode note credential")
	}
	created := uint64(time.Now().Unix())
	modified := created
	metadata := ItemMetadata{Type: NoteItemType}
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
			Credentials: []jsontext.Value{credential},
			Extensions:  []jsontext.Value{extension},
		},
		Metadata: metadata,
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot create note: %w", err)
	}
	return item, nil
}
