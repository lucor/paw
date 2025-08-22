// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


//go:build !windows

package ui

// DetachConsole detaches the console from the current process.
func DetachConsole() {
	// do nothing on non-windows OSes
}
