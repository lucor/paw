// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


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
