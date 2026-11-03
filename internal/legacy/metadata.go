// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"encoding/hex"
	"time"

	"go.lucor.dev/paw/internal/model"
	"golang.org/x/crypto/blake2b"
)

// Metadata contains metadata stored in a legacy Paw item.
type Metadata struct {
	Name     string          `json:"name,omitempty"`
	Subtitle string          `json:"subtitle,omitempty"`
	Type     model.ItemType  `json:"type,omitempty"`
	Modified time.Time       `json:"modified,omitempty"`
	Created  time.Time       `json:"created,omitempty"`
	Favicon  *model.Favicon  `json:"favicon,omitempty"`
	Autofill *model.Autofill `json:"autofill,omitempty"`
}

// ID returns the legacy filename identity derived from the item type and name.
func (m *Metadata) ID() string {
	key := append([]byte(m.Type.String()), []byte(m.Name)...)
	hash, err := blake2b.New256(key)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// GetMetadata returns this legacy metadata value.
func (m *Metadata) GetMetadata() *Metadata { return m }

// String returns the legacy item name.
func (m *Metadata) String() string { return m.Name }
