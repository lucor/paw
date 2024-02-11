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
