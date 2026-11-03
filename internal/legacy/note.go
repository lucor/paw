// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"time"

	"go.lucor.dev/paw/internal/model"
)

// Note is a note decoded from the legacy Paw storage format.
type Note struct {
	Value     string `json:"value,omitempty"`
	*Metadata `json:"metadata,omitempty"`
}

// NewNote creates a legacy note fixture with current metadata.
func NewNote() *Note {
	now := time.Now().UTC()
	return &Note{Metadata: &Metadata{Type: model.NoteItemType, Created: now, Modified: now}}
}
