// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package logging configures Paw's application-wide logger.
package logging

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
)

// Configure sets the application logger to write to the supplied destination.
func Configure(writer io.Writer) error {
	level, err := parseLevel(os.Getenv("PAW_LOG_LEVEL"))
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(writer, &slog.HandlerOptions{Level: level})))
	log.SetOutput(writer)
	return nil
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
