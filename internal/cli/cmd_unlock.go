package cli

import (
	"fmt"
	"os"
	"time"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

// UnlockCmd unlock a vault and starts a session returning its ID
type UnlockCmd struct {
	vaultName string
	life      time.Duration
}

// Name returns the one word command name
func (cmd *UnlockCmd) Name() string {
	return "unlock"
}

// Description returns the command description
func (cmd *UnlockCmd) Description() string {
	return "Unlock a vault returning a session ID"
}

// Usage displays the command usage
func (cmd *UnlockCmd) Usage() {
	template := `Usage: paw-cli session [OPTION] COMMAND VAULT

{{ . }}

Options:
  -t, --lifetime=DURATION   Sets the maximum lifetime for the session. Default to never expire
  -h, --help            Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *UnlockCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flagSet.DurationVar(&cmd.life, "t", 0, "")
	flagSet.DurationVar(&cmd.life, "lifetime", 0, "")

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}

	cmd.vaultName = flagSet.Arg(0)
	return nil
}

// Run runs the command
func (cmd *UnlockCmd) Run(s paw.Storage) error {
	os.Setenv(sessionEnvName, "")
	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}

	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		return err
	}

	sessionID, err := c.Unlock(cmd.vaultName, key, cmd.life)
	if err != nil {
		return err
	}

	fmt.Println("[✓] vault unlocked")
	fmt.Println("Session ID: ", sessionID)
	return nil
}
