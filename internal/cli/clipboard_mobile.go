//go:build android || ios

package cli

import (
	"context"
	"fmt"
)

// initClipboard initializes the clipboard.
// It returns an error if the clipboard is not available to use.
func initClipboard() error {
	return fmt.Errorf("cli clipboard is not supported on this OS")
}

// writeToClipboard writes provided data to clipboard
func writeToClipboard(ctx context.Context, data []byte) error {
	return fmt.Errorf("cli clipboard is not supported on this OS")
}
