// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

// command provides the shared runtime used by CLI command implementations.
// Constructors capture it in closures; urfave owns argument parsing and streams.
type command struct {
	storage        *storage.FileStorage
	vaultID        string
	prompter       *prompter
	out            io.Writer
	errOut         io.Writer
	agentClient    func(string) (agent.PawAgent, error)
	startAgent     func(string) error
	clipboard      func(context.Context, []byte) error
	readFile       func(string) ([]byte, error)
	getenv         func(string) string
	now            func() time.Time
	generateSSHKey func() (*generatedSSHKey, error)
}

type generatedSSHKey struct {
	PrivateKey  string
	PublicKey   string
	Fingerprint string
}

type pawCommand interface {
	Validate(context.Context, *urfave.Command) (context.Context, error)
	Run(context.Context, *urfave.Command) error
}

var (
	_ pawCommand = (*AddCmd)(nil)
	_ pawCommand = (*AgentCmd)(nil)
	_ pawCommand = (*AttachmentCmd)(nil)
	_ pawCommand = (*EditCmd)(nil)
	_ pawCommand = (*ExportCmd)(nil)
	_ pawCommand = (*HistoryCommand)(nil)
	_ pawCommand = (*InitCmd)(nil)
	_ pawCommand = (*ListCmd)(nil)
	_ pawCommand = (*LockCmd)(nil)
	_ pawCommand = (*PwGenCmd)(nil)
	_ pawCommand = (*RemoveCmd)(nil)
	_ pawCommand = (*RestoreCmd)(nil)
	_ pawCommand = (*ShowCmd)(nil)
	_ pawCommand = (*SyncCmd)(nil)
	_ pawCommand = (*UnlockCmd)(nil)
	_ pawCommand = (*VersionCmd)(nil)
)

// validateArgs checks that the number of positional arguments is within bounds.
func validateArgs(c *urfave.Command, min, max int) error {
	if n := c.Args().Len(); n < min {
		return invalidArgumentsError("invalid arguments: missing required arguments: %s\nUsage: %s %s", strings.TrimSpace(c.ArgsUsage), c.FullName(), strings.TrimSpace(c.ArgsUsage))
	} else if n > max {
		return invalidArgumentsError("invalid arguments: unexpected arguments; usage: %s %s", c.FullName(), strings.TrimSpace(c.ArgsUsage))
	}
	return nil
}

// Printf writes result output to stdout.
func (base *command) Printf(format string, args ...any) error {
	_, err := fmt.Fprintf(base.out, format, args...)
	return err
}

// Stdout returns the command's result writer.
func (base *command) Stdout() io.Writer {
	return base.out
}

// Infof writes operational information to stderr.
func (base *command) Infof(format string, args ...any) error {
	_, err := fmt.Fprintf(base.errOut, format, args...)
	return err
}

// Warnf writes a warning to stderr.
func (base *command) Warnf(format string, args ...any) error {
	_, err := fmt.Fprintf(base.errOut, format, args...)
	return err
}

// loadVault loads the selected vault.
func (base *command) loadVault(key *paw.Key) (*model.Vault, error) {
	return base.storage.LoadVault(base.vaultID, key)
}
