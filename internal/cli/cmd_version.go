package cli

import (
	"fmt"

	"lucor.dev/paw/internal/paw"
)

// Version is the version command
type VersionCmd struct{}

// Name returns the one word command name
func (cmd *VersionCmd) Name() string {
	return "version"
}

// Description returns the command description
func (cmd *VersionCmd) Description() string {
	return "Print the version information"
}

// Usage displays the command usage
func (cmd *VersionCmd) Usage() {
	template := `Usage: paw cli version

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *VersionCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)

	return nil
}

// Run runs the command
func (cmd *VersionCmd) Run(s paw.Storage) error {
	fmt.Printf("paw cli version %s\n", paw.Version())
	return nil
}
