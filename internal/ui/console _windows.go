// Copyright 2024 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build windows

package ui

import "syscall"

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	procFreeConsole = kernel32.NewProc("FreeConsole")
)

// DetachConsole detaches the console from the current process.
func DetachConsole() {
	procFreeConsole.Call()
}
