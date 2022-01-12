package cli

import (
	"fmt"
	"runtime/debug"
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

// Run runs the command
func (cmd *VersionCmd) Run() error {
	fmt.Printf("paw-cli version %s\n", getVersion())
	return nil
}

// Parse parses the arguments and set the usage for the command
func (cmd *VersionCmd) Parse(args []string) error {
	return nil
}

// Usage displays the command usage
func (cmd *VersionCmd) Usage() {
	template := `Usage: paw-cli version

{{ . }}
`
	printUsage(template, cmd.Description())
}

func getVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return version
}
