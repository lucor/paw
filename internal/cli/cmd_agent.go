package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

const (
	agentStartSubCmd    = "start"
	agentSessionsSubCmd = "sessions"
)

// Agent manages the Paw agent
type AgentCmd struct {
	command string
}

// Name returns the one word command name
func (cmd *AgentCmd) Name() string {
	return "agent"
}

// Description returns the command description
func (cmd *AgentCmd) Description() string {
	return "Manages the Paw agent"
}

// Usage displays the command usage
func (cmd *AgentCmd) Usage() {
	template := `Usage: paw cli agent COMMAND

{{ . }}

Commands:
  start       Starts the agent
  sessions    Show the active sessions

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *AgentCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	cmd.command = flagSet.Arg(0)
	if cmd.command != agentStartSubCmd && cmd.command != agentSessionsSubCmd {
		cmd.Usage()
		os.Exit(1)
	}

	return nil
}

// Run runs the command
func (cmd *AgentCmd) Run(s paw.Storage) error {
	switch cmd.command {
	case agentStartSubCmd:
		c, err := agent.NewClient(s.SocketAgentPath())
		if err == nil {
			t, err := c.Type()
			if err == nil {
				fmt.Printf("[✗] agent of type %s is already running\n", t)
				os.Exit(1)
			}
		}

		a := agent.NewCLI()
		defer a.Close()
		agent.Run(a, s.SocketAgentPath())
	case agentSessionsSubCmd:
		c, err := agent.NewClient(s.SocketAgentPath())
		if err != nil {
			return err
		}
		sessions, err := c.Sessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			fmt.Println("No session found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 1, ' ', tabwriter.Debug)
		fmt.Fprintln(w, "Session ID\tVault\tLifetime")
		for _, session := range sessions {
			fmt.Fprintf(w, "%s\t%s\t%s\n", session.ID, session.Vault, session.Lifetime.Round(1*time.Second))
		}
		w.Flush()
	}
	return nil
}
