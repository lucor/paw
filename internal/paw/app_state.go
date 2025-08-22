// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import "time"

// AppState represents the application state
type AppState struct {
	// Modified is the last time the state was modified, example: preferences changed or vaults modified
	Modified time.Time `json:"modified,omitempty"`
	// Preferences contains the application preferences
	Preferences *Preferences `json:"preferences,omitempty"`
}
