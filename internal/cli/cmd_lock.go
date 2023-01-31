package cli

import (
	"fmt"
	"os"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

// Lock locks a Paw vault removing all the associated sessions from the agent
type LockCmd struct {
	vaultName string
}

// Name returns the one word command name
func (cmd *LockCmd) Name() string {
	return "lock"
}

// Description returns the command description
func (cmd *LockCmd) Description() string {
	return "Lock a vault"
}

// Usage displays the command usage
func (cmd *LockCmd) Usage() {
	template := `Usage: paw-cli lock [OPTION] VAULT

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *LockCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	cmd.vaultName = flagSet.Arg(0)
	return nil
}

// Run runs the command
func (cmd *LockCmd) Run(s paw.Storage) error {
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		return err
	}
	err = c.Lock(cmd.vaultName)
	if err != nil {
		return err
	}

	fmt.Println("[✓] vault locked")
	return nil
}
