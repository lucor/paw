// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/paw"
)

func sessionFlag() urfave.Flag {
	return &urfave.StringFlag{
		Name:        "session",
		HideDefault: true,
		Usage:       "Session ID to use instead of prompting for the vault password",
		Sources:     urfave.EnvVars(paw.ENV_SESSION),
	}
}
