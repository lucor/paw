// Copyright 2024 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build windows

package agent

import (
	"net"
	"time"

	npipe "gopkg.in/natefinch/npipe.v2"
)

// dialWithTimeout dials the named pipe with a timeout
func dialWithTimeout(socketPath string, timeout time.Duration) (net.Conn, error) {
	return npipe.DialTimeout(socketPath, timeout)
}

// listen listens on the named pipe
func listen(socketPath string) (net.Listener, error) {
	return npipe.Listen(socketPath)
}
