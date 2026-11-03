// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package legacy reads the pre-CXF Paw storage format for migration.
package legacy

// Item is an item decoded from the legacy Paw storage format.
type Item interface {
	ID() string
	GetMetadata() *Metadata
}
