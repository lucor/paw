package cli

import (
	"fmt"
	"os"
	"runtime/debug"

	"lucor.dev/paw/internal/paw"
)

const version = "develop"

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
	template := `Usage: paw-cli version

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *VersionCmd) Parse(args []string) error {
	flags, err := newCommonFlags()
	if err != nil {
		return err
	}

	flagSet.Parse(args)
	if flags.Help {
		cmd.Usage()
		os.Exit(0)
	}
	return nil
}

// Run runs the command
func (cmd *VersionCmd) Run(s paw.Storage) error {
	fmt.Printf("paw-cli version %s\n", getVersion())
	return nil
}

func getVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return version
}
