// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
	now := time.Now()
	return &Note{
		Metadata: &Metadata{
			Type:     NoteItemType,
			Created:  now,
			Modified: now,
		},
	}
}
