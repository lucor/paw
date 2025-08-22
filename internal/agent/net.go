// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package agent

import (
	"net"
	"time"
)

// dialWithTimeout dials the unix socket with a timeout
func dialWithTimeout(socketPath string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, timeout)
}

// listen listens on the unix socket
func listen(socketPath string) (net.Listener, error) {
	return net.Listen("unix", socketPath)
}
