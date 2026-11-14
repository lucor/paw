// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package logging configures Paw's application-wide logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"go.lucor.dev/paw/internal/paw"
)

// Init configures the default logger from the PAW_LOG_LEVEL and PAW_LOG_FILE environment variables.
func Init() (io.Closer, error) {
	level, err := parseLevel(os.Getenv("PAW_LOG_LEVEL"))
	if err != nil {
		return nil, err
	}
	path := os.Getenv("PAW_LOG_FILE")
	if path == "" {
		parent := os.Getenv(paw.ENV_HOME)
		if parent == "" {
			parent, err = os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("get Paw home directory: %w", err)
			}
		}
		path = filepath.Join(parent, ".paw", "paw.log")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create Paw log directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open Paw log file: %w", err)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: level})))
	return file, nil
}

func parseLevel(value string) (slog.Level, error) {
	switch strings.ToLower(value) {
	case "", "warn":
		return slog.LevelWarn, nil
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid PAW_LOG_LEVEL %q: expected debug, info, warn, or error", value)
	}
}
