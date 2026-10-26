// Package migration converts legacy Paw data to the current storage format.
package migration

import (
	"encoding/json"
	"fmt"
	"unicode/utf8"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

const itemExtensionName = "paw.pm/item"

// noteExtension holds note metadata that CXF does not represent.
type noteExtension struct {
	// Name identifies the Paw extension.
	Name string `json:"name"`
	// Type identifies the original Paw item type.
	Type paw.ItemType `json:"type"`
}

// MigrateNote converts a legacy Paw note to a CXF-backed Paw item.
func MigrateNote(note *paw.Note) (*model.Item, error) {
	if note == nil || note.Metadata == nil || note.Type != paw.NoteItemType {
		return nil, fmt.Errorf("cannot migrate note: invalid metadata")
	}
	if !utf8.ValidString(note.Subtitle) {
		return nil, fmt.Errorf("cannot migrate note: invalid UTF-8")
	}
	item, err := model.NewNote(note.Name, note.Value)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate note: %w", err)
	}
	extension, err := json.Marshal(noteExtension{
		Name: itemExtensionName,
		Type: paw.NoteItemType,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot encode note metadata")
	}
	item.CreationAt, err = timestamp(note.Created)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate note creation date: %w", err)
	}
	item.ModifiedAt, err = timestamp(note.Modified)
	if err != nil {
		return nil, fmt.Errorf("cannot migrate note modification date: %w", err)
	}
	item.Extensions = []json.RawMessage{extension}
	if note.Subtitle != "" {
		subtitle := note.Subtitle
		item.Subtitle = &subtitle
	}
	if err := item.Validate(); err != nil {
		return nil, fmt.Errorf("cannot migrate note: %w", err)
	}
	return item, nil
}
