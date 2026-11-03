// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"time"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// Password is a password decoded from the legacy Paw storage format.
type Password struct {
	Value     string           `json:"value,omitempty"`
	Format    paw.Format       `json:"format,omitempty"`
	Length    int              `json:"length,omitempty"`
	Mode      paw.PasswordMode `json:"mode,omitempty"`
	*Metadata `json:"metadata,omitempty"`
	*Note     `json:"note,omitempty"`
}

// NewPassword creates a legacy password fixture with current metadata.
func NewPassword() *Password {
	now := time.Now().UTC()
	return &Password{
		Metadata: &Metadata{Type: model.PasswordItemType, Created: now, Modified: now},
		Note:     NewNote(),
	}
}
