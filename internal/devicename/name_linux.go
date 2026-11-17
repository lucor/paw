// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !android

package devicename

import (
	"os"
	"strconv"
	"strings"
)

const fallback = "Linux"

func detect() string {
	if name := hostname(); Validate(name) == nil {
		return name
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fallback
	}
	name := distributionName(string(data))
	if Validate(name) == nil {
		return name
	}
	return fallback
}

func distributionName(data string) string {
	for line := range strings.SplitSeq(data, "\n") {
		value, ok := strings.CutPrefix(line, "NAME=")
		if !ok {
			continue
		}
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		return strings.TrimSpace(value)
	}
	return ""
}
