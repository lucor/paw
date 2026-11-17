// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package devicename

const fallback = "Android"

func detect() string {
	name := command("/system/bin/getprop", "ro.product.model")
	if Validate(name) == nil {
		return name
	}
	return fallback
}
