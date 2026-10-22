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

// Agent manages the Paw agent
type AgentCmd struct {
	*command
	agentCommand string
}

func newAgentCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:   "agent",
		Usage:  "Manages the Paw agent",
		Action: requireSubcommand,
		Commands: []*urfave.Command{
			{Name: "start", Usage: "Starts the agent", ArgsUsage: " ", Action: withArgs(0, 0, func(ctx context.Context, c *urfave.Command) error {
				base := base.forInvocation(c)
				return (&AgentCmd{command: base, agentCommand: agentStartSubCmd}).Run(ctx)
			})},
			{Name: "sessions", Usage: "Lists active sessions", ArgsUsage: " ", Action: withArgs(0, 0, func(ctx context.Context, c *urfave.Command) error {
				base := base.forInvocation(c)
				return (&AgentCmd{command: base, agentCommand: agentSessionsSubCmd}).Run(ctx)
			})},
		},
	}
}

// Run runs the command
func (cmd *AgentCmd) Run(ctx context.Context) error {
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

		return base.startAgent(s.SocketAgentPath())
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
			fmt.Fprintln(cmd.out, "No session found")
			return nil
		}

		w := tabwriter.NewWriter(cmd.out, 0, 0, 1, ' ', tabwriter.Debug)
		fmt.Fprintln(w, "Session ID\tVault\tLifetime")
		for _, session := range sessions {
			fmt.Fprintf(w, "%s\t%s\t%s\n", session.ID, session.Vault, session.Lifetime.Round(1*time.Second))
		}
		w.Flush()
	}
	return nil
}
