// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package cli

import (
	"flag"
	"fmt"
	"io"
	"os"

	"lucor.dev/paw/internal/paw"
)

var flagSet = flag.NewFlagSet("paw", flag.ContinueOnError)

// CommonFlags holds the flags shared between all commands
type CommonFlags struct {
	// Help displays the command help and exit
	Help bool
	// SessionID is the session ID
	SessionID string
}

type flagOpts struct {
	Session bool
}

// newCommonFlags defines all the flags for the shared options
func newCommonFlags(o flagOpts) (*CommonFlags, error) {
	flags := &CommonFlags{}
	flagSet.BoolVar(&flags.Help, "help", false, "")
	flagSet.BoolVar(&flags.Help, "h", false, "")
	if o.Session {
		flagSet.StringVar(&flags.SessionID, "session", "", "")
	}
	return flags, nil
}

// SetEnv sets the env variables according to the flag values
func (f *CommonFlags) SetEnv() {
	if f.SessionID != "" {
		os.Setenv(paw.ENV_SESSION, f.SessionID)
	}
}

func (f *CommonFlags) Parse(cmd Cmd, args []string) {
	flagSet.SetOutput(io.Discard)
	err := flagSet.Parse(args)
	if err != nil {
		fmt.Println("[✗]", err)
		fmt.Println()
		cmd.Usage()
		os.Exit(1)
	}
	if f.Help {
		cmd.Usage()
		os.Exit(0)
	}
}
