package main

import (
	"fmt"
	"os"

	"lucor.dev/paw/internal/cli"
	"lucor.dev/paw/internal/paw"
)

// Version allow to set the version at link time
var Version string

func main() {
	s, err := paw.NewOSStorage()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] %s\n", err)
		os.Exit(1)
	}

	// Define the command to use
	commands := []cli.Cmd{
		&cli.AgentCmd{},
		&cli.AddCmd{},
		&cli.EditCmd{},
		&cli.InitCmd{},
		&cli.ListCmd{},
		&cli.LockCmd{},
		&cli.PwGenCmd{},
		&cli.RemoveCmd{},
		&cli.ShowCmd{},
		&cli.UnlockCmd{},
		&cli.VersionCmd{Version: Version},
	}

	// display the usage if no command is specified
	if len(os.Args) == 1 {
		cli.Usage(commands)
		os.Exit(1)
	}

	// check for valid command
	var cmd cli.Cmd
	for _, v := range commands {
		if os.Args[1] == v.Name() {
			cmd = v
			break
		}
	}

	// If no valid command is specified display the usage
	if cmd == nil {
		cli.Usage(commands)
		os.Exit(1)
	}

	// Parse the arguments for the command
	// It will display the command usage if -help is specified
	// and will exit in case of error
	err = cmd.Parse(os.Args[2:])
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] %s\n", err)
		os.Exit(1)
	}

	// Finally run the command
	err = cmd.Run(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] %s\n", err)
		os.Exit(1)
	}
}
