// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"bytes"
	"io"
	"log"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
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

func TestConfigureUsesWriterAndWarnDefault(t *testing.T) {
	withCleanLogger(t)
	t.Setenv("PAW_LOG_LEVEL", "")
	var output bytes.Buffer
	require.NoError(t, Configure(&output))
	slog.Debug("hidden")
	slog.Warn("visible")
	require.NotContains(t, output.String(), "hidden")
	require.Contains(t, output.String(), "visible")
}

func TestConfigureRedirectsLegacyLogger(t *testing.T) {
	withCleanLogger(t)
	var output bytes.Buffer
	t.Setenv("PAW_LOG_LEVEL", "debug")
	require.NoError(t, Configure(&output))
	log.Print("first legacy message")
	slog.Debug("debug message")
	require.Contains(t, output.String(), "first legacy message")
	require.Contains(t, output.String(), "debug message")
}

func TestConfigureRejectsInvalidLevel(t *testing.T) {
	withCleanLogger(t)
	t.Setenv("PAW_LOG_LEVEL", "verbose")
	err := Configure(io.Discard)
	require.ErrorContains(t, err, "invalid PAW_LOG_LEVEL")
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
