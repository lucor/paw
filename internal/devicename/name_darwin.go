// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package devicename

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const fallback = "macOS"

func detect() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/usr/sbin/scutil", "--get", "ComputerName").Output()
	if err == nil {
		name := strings.TrimSpace(string(out))
		if Validate(name) == nil {
			return name
		}
	}
	name := hostname()
	if Validate(name) == nil {
		return name
	}
	return fallback
}
