// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"io"
	"time"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

// command provides the shared runtime used by CLI command implementations.
// Constructors capture it in closures; urfave owns argument parsing and streams.
type command struct {
	storage        paw.Storage
	prompter       *prompter
	out            io.Writer
	errOut         io.Writer
	agentClient    func(string) (agent.PawAgent, error)
	startAgent     func(string) error
	clipboard      func(context.Context, []byte) error
	readFile       func(string) ([]byte, error)
	getenv         func(string) string
	now            func() time.Time
	oneTimeKey     func() (*paw.Key, error)
	generateSSHKey func() (*paw.SSHKey, error)
}

// forInvocation binds streams after urfave has applied its inherited defaults.
// A supplied prompter is retained so tests can inject password input too.
func (cmd *command) forInvocation(c *urfave.Command) *command {
	invocation := *cmd
	invocation.out = c.Writer
	invocation.errOut = c.ErrWriter
	if invocation.prompter == nil {
		invocation.prompter = newPrompter(c.Reader, c.ErrWriter)
	}
	return &invocation
}
