// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"time"
)

// Declare conformity to Item interface
var _ Item = (*Note)(nil)

type Note struct {
	Value     string `json:"value,omitempty"`
	*Metadata `json:"metadata,omitempty"`
}

func NewNote() *Note {
	now := time.Now().UTC()
	return &Note{
		Metadata: &Metadata{
			Type:     NoteItemType,
			Created:  now,
			Modified: now,
		},
	}
}
