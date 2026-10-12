// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build darwin || linux || windows

package cli

import (
	"bytes"
	"context"
	"errors"
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
	last, err := clipboard.Read(ctx, clipboard.FmtText)
	if err != nil && !errors.Is(err, clipboard.ErrNoData) {
		return fmt.Errorf("unable to read data from clipboard: %w", err)
	}
	if bytes.Equal(last, data) {
		// data is the same in clipboard no need to write
		return nil
	}

	changed, err := clipboard.Write(ctx, clipboard.FmtText, data)
	if err != nil {
		return fmt.Errorf("unable to write data to clipboard: %w", err)
	}

	ti := time.NewTimer(clipboardWatchInterval)
	defer ti.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("unable to write data to clipboard: timeout reached")
	case <-changed:
		return fmt.Errorf("clipboard has been overwritten by others and data is lost")
	case <-ti.C:
		return nil
	}
}
