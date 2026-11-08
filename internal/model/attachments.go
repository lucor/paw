// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"

	"go.lucor.dev/cxf"
)

// NewFileID returns a random CXF-compatible file identifier.
func NewFileID() (string, error) {
	id, err := newID()
	if err != nil {
		return "", fmt.Errorf("cannot generate file ID: %w", err)
	}
	return id, nil
}

// Files returns all file credentials in their original order.
func (item *Item) Files() ([]cxf.File, error) {
	if item == nil {
		return nil, fmt.Errorf("item is nil")
	}
	files := make([]cxf.File, 0)
	ids := make(map[string]struct{})
	for _, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return nil, fmt.Errorf("cannot read credential type: %w", err)
		}
		if typeName != cxf.CredentialTypeFile {
			continue
		}
		var file cxf.File
		if err := json.Unmarshal(raw, &file); err != nil {
			return nil, fmt.Errorf("cannot decode file credential: %w", err)
		}
		if _, ok := ids[file.ID]; ok {
			return nil, fmt.Errorf("duplicate file credential ID")
		}
		ids[file.ID] = struct{}{}
		files = append(files, file)
	}
	return files, nil
}

// AddFile appends a validated file credential to a Note or Login item.
func (item *Item) AddFile(file cxf.File) error {
	if err := item.validateFileCategory("add"); err != nil {
		return err
	}
	if err := file.Validate(); err != nil {
		return fmt.Errorf("invalid file credential: %w", err)
	}
	files, err := item.Files()
	if err != nil {
		return err
	}
	for _, existing := range files {
		if existing.ID == file.ID {
			return fmt.Errorf("file credential ID already exists")
		}
	}
	encoded, err := json.Marshal(file, json.Deterministic(true))
	if err != nil {
		return fmt.Errorf("cannot encode file credential: %w", err)
	}
	item.Credentials = append(item.Credentials, jsontext.Value(encoded))
	return nil
}

// RemoveFile removes a file credential by ID from a Note or Login item.
func (item *Item) RemoveFile(id string) error {
	if err := item.validateFileCategory("remove"); err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("file ID is required")
	}
	files, err := item.Files()
	if err != nil {
		return err
	}
	found := false
	for _, file := range files {
		if file.ID == id {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("file credential does not exist")
	}
	credentials := make([]jsontext.Value, 0, len(item.Credentials)-1)
	for _, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			return fmt.Errorf("cannot read credential type: %w", err)
		}
		if typeName == cxf.CredentialTypeFile {
			var file cxf.File
			if err := json.Unmarshal(raw, &file); err != nil {
				return fmt.Errorf("cannot decode file credential: %w", err)
			}
			if file.ID == id {
				continue
			}
		}
		credentials = append(credentials, raw)
	}
	item.Credentials = credentials
	return nil
}

func (item *Item) validateFileCategory(operation string) error {
	if item == nil {
		return fmt.Errorf("cannot %s file: item is nil", operation)
	}
	if item.Metadata.Type != NoteItemType && item.Metadata.Type != LoginItemType {
		return fmt.Errorf("cannot %s file for %s item", operation, item.Metadata.Type.Label())
	}
	return nil
}
