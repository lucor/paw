// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/paw"
)

func newVersionCommand() *urfave.Command {
	return &urfave.Command{
		Name:      "version",
		Usage:     "Prints version information",
		ArgsUsage: " ",
		Action: withArgs(0, 0, func(ctx context.Context, c *urfave.Command) error {
			_, err := fmt.Fprintf(c.Writer, "paw cli version %s\n", paw.Version())
			return err
		}),
	}
}
