// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package devicename

const fallback = "Windows"

func detect() string {
	if name := hostname(); Validate(name) == nil {
		return name
	}
	return fallback
}
