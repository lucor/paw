//go:build !windows

package ui

// DetachConsole detaches the console from the current process.
func DetachConsole() {
	// do nothing on non-windows OSes
}
