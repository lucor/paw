// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package devicename detects a readable name for the local device.
package devicename

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Detect returns a readable local device name.
func Detect() string { return detect() }

// Validate checks that a user-provided name fits in a DNS-SD TXT record.
func Validate(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "localhost") || strings.EqualFold(value, "localhost.localdomain") {
		return errors.New("enter a device name other than localhost")
	}
	if len(value) > 200 {
		return errors.New("device name must be at most 200 bytes")
	}
	return nil
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil {
		return ""
	}
	return normalizedHostname(name)
}

func normalizedHostname(name string) string {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, "localhost") || strings.EqualFold(name, "localhost.localdomain") {
		return ""
	}
	return name
}

func command(path string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
