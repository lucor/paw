// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui"
)

func TestDefaultWindowDimensions(t *testing.T) {
	require.Equal(t, 400, ui.DefaultWindowWidth)
	require.Equal(t, 640, ui.DefaultWindowHeight)
}

func TestMakeLogger(t *testing.T) {
	t.Cleanup(func() { slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil))) })
	t.Run("falls back to stderr", func(t *testing.T) {
		blocked := filepath.Join(t.TempDir(), "blocked")
		require.NoError(t, os.WriteFile(blocked, nil, 0600))
		t.Setenv("PAW_LOG_FILE", filepath.Join(blocked, "paw.log"))
		read, write, err := os.Pipe()
		require.NoError(t, err)
		original := os.Stderr
		os.Stderr = write
		t.Cleanup(func() { os.Stderr = original })
		logger := makeLogger()
		require.Nil(t, logger)
		require.NoError(t, write.Close())
		output, err := io.ReadAll(read)
		require.NoError(t, err)
		require.Contains(t, string(output), "could not initialize file logging")
	})
	t.Run("opens the configured file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "paw.log")
		t.Setenv("PAW_LOG_FILE", path)
		logger := makeLogger()
		require.NotNil(t, logger)
		require.NoError(t, logger.Close())
		_, err := os.Stat(path)
		require.NoError(t, err)
	})
}
