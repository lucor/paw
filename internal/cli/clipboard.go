//go:build !android && !ios

package cli

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"golang.design/x/clipboard"
)

const (
	clipboardWatchInterval = 10 * time.Millisecond
)

// initClipboard initializes the clipboard.
// It returns an error if the clipboard is not available to use.
func initClipboard() error {
	return clipboard.Init()
}

// writeToClipboard writes provided data to clipboard
func writeToClipboard(ctx context.Context, data []byte) error {
	last := clipboard.Read(clipboard.FmtText)
	if bytes.Equal(last, data) {
		// data is the same in clipboard no need to write
		return nil
	}

	clipboard.Write(clipboard.FmtText, data)

	ti := time.NewTicker(clipboardWatchInterval)
	defer ti.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("unable to write data to clipboard: timeout reached")
		case <-ti.C:
			b := clipboard.Read(clipboard.FmtText)
			if b == nil {
				continue
			}
			if bytes.Equal(last, b) {
				// clipboard data not changed
				continue
			}
			if !bytes.Equal(b, data) {
				// clipboard data changed but with unexpected content
				return fmt.Errorf("clipboard has been overwritten by others and data is lost")
			}
			return nil
		}
	}
}
