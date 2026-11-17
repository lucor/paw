// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package devicename

const fallback = "macOS"

func detect() string {
	name := command("/usr/sbin/scutil", "--get", "ComputerName")
	if Validate(name) == nil {
		return name
	}
	name = hostname()
	if Validate(name) == nil {
		return name
	}
	return fallback
}
