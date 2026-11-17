// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !android && !darwin && !linux && !windows

package devicename

const fallback = "Paw device"

func detect() string {
	if name := hostname(); name != "" {
		return name
	}
	return fallback
}
