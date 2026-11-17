// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package devicename

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

const fallback = "Android"

func detect() string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "/system/bin/getprop", "ro.product.model").Output()
	if err != nil {
		return fallback
	}
	name := strings.TrimSpace(string(out))
	if Validate(name) == nil {
		return name
	}
	return fallback
}
