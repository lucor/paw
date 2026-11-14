// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/paw"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		value string
		level slog.Level
	}{
		{value: "", level: slog.LevelWarn},
		{value: "warn", level: slog.LevelWarn},
		{value: "DEBUG", level: slog.LevelDebug},
		{value: "Info", level: slog.LevelInfo},
		{value: "error", level: slog.LevelError},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			level, err := parseLevel(test.value)
			require.NoError(t, err)
			require.Equal(t, test.level, level)
		})
	}
}

func TestParseLevelRejectsInvalidValue(t *testing.T) {
	_, err := parseLevel("verbose")
	require.EqualError(t, err, `invalid PAW_LOG_LEVEL "verbose": expected debug, info, warn, or error`)
}

func TestInitDefaultsToWarnInLogFile(t *testing.T) {
	withCleanLogger(t)
	root := t.TempDir()
	t.Setenv(paw.ENV_HOME, root)
	t.Setenv("PAW_LOG_LEVEL", "")
	t.Setenv("PAW_LOG_FILE", "")
	closer, err := Init()
	require.NoError(t, err)
	require.NotNil(t, closer)
	slog.Debug("hidden")
	slog.Warn("visible")
	require.NoError(t, closer.Close())
	content, err := os.ReadFile(filepath.Join(root, ".paw", "paw.log"))
	require.NoError(t, err)
	require.NotContains(t, string(content), "hidden")
	require.Contains(t, string(content), "visible")
}

func TestInitCreatesRestrictiveAppendOnlyFileAndRedirectsLegacyLogger(t *testing.T) {
	withCleanLogger(t)
	oldStderr := os.Stderr
	stderrReader, stderrWriter, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = stderrWriter
	t.Cleanup(func() { os.Stderr = oldStderr })
	path := filepath.Join(t.TempDir(), "paw.log")
	t.Setenv("PAW_LOG_LEVEL", "debug")
	t.Setenv("PAW_LOG_FILE", path)
	closer, err := Init()
	require.NoError(t, err)
	log.Print("first legacy message")
	slog.Debug("debug message")
	require.NoError(t, stderrWriter.Close())
	stderr, err := io.ReadAll(stderrReader)
	require.NoError(t, err)
	require.Empty(t, stderr)
	require.NoError(t, closer.Close())
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "first legacy message")
	require.Contains(t, string(content), "debug message")

	closer, err = Init()
	require.NoError(t, err)
	log.Print("second legacy message")
	require.NoError(t, closer.Close())
	content, err = os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "first legacy message")
	require.Contains(t, string(content), "second legacy message")
}

func TestInitRejectsInvalidLevelAndUnavailableFile(t *testing.T) {
	withCleanLogger(t)
	t.Setenv("PAW_LOG_LEVEL", "verbose")
	t.Setenv("PAW_LOG_FILE", "")
	_, err := Init()
	require.ErrorContains(t, err, "invalid PAW_LOG_LEVEL")

	t.Setenv("PAW_LOG_LEVEL", "warn")
	t.Setenv("PAW_LOG_FILE", t.TempDir())
	_, err = Init()
	require.ErrorContains(t, err, "open Paw log file")
}

func withCleanLogger(t *testing.T) {
	t.Helper()
	oldLogger := slog.Default()
	oldOutput := log.Writer()
	t.Cleanup(func() {
		slog.SetDefault(oldLogger)
		log.SetOutput(oldOutput)
	})
}
