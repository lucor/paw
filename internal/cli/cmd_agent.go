// SPDX-FileCopyrightText: 2023-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"text/tabwriter"
	"time"

	urfave "github.com/urfave/cli/v3"
)

const (
	agentStartSubCmd    = "start"
	agentSessionsSubCmd = "sessions"
)

// AgentCmd manages the Paw agent
type AgentCmd struct {
	*command
	agentCommand string
}

// newAgentCommand constructs the agent-start command.
func newAgentCommand(base *command) *urfave.Command {
	start := &AgentCmd{command: base, agentCommand: agentStartSubCmd}
	sessions := &AgentCmd{command: base, agentCommand: agentSessionsSubCmd}
	return &urfave.Command{
		Name:   "agent",
		Usage:  "Manages the Paw agent",
		Action: commandGroupAction,
		Commands: []*urfave.Command{
			newAgentSubcommand("start", "Starts the agent", start),
			newAgentSubcommand("sessions", "Lists active sessions", sessions),
		},
		OnUsageError: onUsageError,
	}
}

// newAgentSubcommand registers one agent operation.
func newAgentSubcommand(name, usage string, cmd *AgentCmd) *urfave.Command {
	c := &urfave.Command{
		Name:         name,
		Usage:        usage,
		ArgsUsage:    " ",
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks that an agent operation has no positional arguments.
func (cmd *AgentCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 0); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// Run starts the Paw agent using the selected socket path.
func (cmd *AgentCmd) Run(ctx context.Context, _ *urfave.Command) error {
	base := cmd.command
	s := base.storage
	switch cmd.agentCommand {
	case agentStartSubCmd:
		c, err := base.agentClient(s.SocketAgentPath())
		if err == nil {
			t, err := c.Type()
			if err == nil {
				return fmt.Errorf("agent of type %s is already running", t)
			}
		}

		socketPath, err := s.PrepareAgentSocket()
		if err != nil {
			return fmt.Errorf("could not prepare agent socket: %w", err)
		}
		return base.startAgent(socketPath)
	case agentSessionsSubCmd:
		c, err := base.agentClient(s.SocketAgentPath())
		if err != nil {
			return err
		}
		sessions, err := c.Sessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			return nil
		}

		vaultIDs := make([]string, len(sessions))
		for i, session := range sessions {
			vaultIDs[i] = session.Vault
		}
		prefixes := vaultIDPrefixes(vaultIDs)
		w := tabwriter.NewWriter(base.Stdout(), 0, 0, 1, ' ', 0)
		if _, err := fmt.Fprintln(w, "Session ID\tVault ID\tLifetime"); err != nil {
			return err
		}
		for _, session := range sessions {
			if _, err := fmt.Fprintf(w, "%s\t%s\t%s\n", session.ID, prefixes[session.Vault], session.Lifetime.Round(time.Second)); err != nil {
				return err
			}
		}
		return w.Flush()
	}
	return nil
}
