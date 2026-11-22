// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"io"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/paw"
)

type VersionCmd struct {
	version string
	writer  io.Writer
}

// newVersionCommand constructs the version-display command.
func newVersionCommand() *urfave.Command {
	cmd := &VersionCmd{version: paw.Version()}
	c := &urfave.Command{
		Name:         "version",
		Usage:        "Prints version information",
		ArgsUsage:    " ",
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks that version has no positional arguments.
func (cmd *VersionCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 0); err != nil {
		return ctx, err
	}
	cmd.writer = c.Writer
	return ctx, nil
}

// Run writes the CLI version to the configured output.
func (cmd *VersionCmd) Run(ctx context.Context, _ *urfave.Command) error {
	_, err := fmt.Fprintf(cmd.writer, "paw cli version %s\n", cmd.version)
	return err
}
