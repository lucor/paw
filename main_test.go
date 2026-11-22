// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/cli"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui"
)

func TestDefaultWindowDimensions(t *testing.T) {
	require.Equal(t, 400, ui.DefaultWindowWidth)
	require.Equal(t, 640, ui.DefaultWindowHeight)
}

func TestRunPrintsCLIErrorsToStderr(t *testing.T) {
	previousArgs := os.Args
	previousStdout, previousStderr := os.Stdout, os.Stderr
	previousLogger, previousLogOutput := slog.Default(), log.Writer()
	t.Cleanup(func() {
		os.Args = previousArgs
		os.Stdout, os.Stderr = previousStdout, previousStderr
		slog.SetDefault(previousLogger)
		log.SetOutput(previousLogOutput)
	})

	root := t.TempDir()
	t.Setenv(paw.ENV_HOME, root)
	os.Args = []string{"paw", "cli", "unknown-command"}
	stdout, err := os.CreateTemp(root, "stdout-*")
	require.NoError(t, err)
	defer stdout.Close()
	stderr, err := os.CreateTemp(root, "stderr-*")
	require.NoError(t, err)
	defer stderr.Close()
	os.Stdout, os.Stderr = stdout, stderr

	require.Equal(t, cli.ExitCodeInvalidArguments, run())
	require.NoError(t, stdout.Sync())
	require.NoError(t, stderr.Sync())
	stdoutContent, err := os.ReadFile(stdout.Name())
	require.NoError(t, err)
	stderrContent, err := os.ReadFile(stderr.Name())
	require.NoError(t, err)
	require.Empty(t, stdoutContent)
	require.Equal(t, "Error: unknown command \"unknown-command\" for paw cli\n", string(stderrContent))
}

func TestConfigureLoggerDoesNotWriteFileOnMobile(t *testing.T) {
	previousLogger := slog.Default()
	previousOutput := log.Writer()
	t.Cleanup(func() {
		slog.SetDefault(previousLogger)
		log.SetOutput(previousOutput)
	})

	root := t.TempDir()
	store, err := storage.NewFileStorage(root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	logPath := filepath.Join(root, "mobile.log")
	t.Setenv("PAW_LOG_FILE", logPath)
	t.Setenv("PAW_LOG_LEVEL", "")

	var stderr bytes.Buffer
	require.Nil(t, configureLogger(store, true, &stderr))
	slog.Warn("mobile diagnostic")
	require.Contains(t, stderr.String(), "mobile diagnostic")
	_, err = os.Stat(logPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}
