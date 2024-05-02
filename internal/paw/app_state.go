// Copyright 2024 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

import "time"

// AppState represents the application state
type AppState struct {
	// Modified is the last time the state was modified, example: preferences changed or vaults modified
	Modified time.Time `json:"modified,omitempty"`
	// Preferences contains the application preferences
	Preferences *Preferences `json:"preferences,omitempty"`
}
