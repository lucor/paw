package main

import (
	"log"
	"os"

	"lucor.dev/paw/internal/cli"
)

func main() {

	log.SetFlags(0)

	// Define the command to use
	commands := []cli.Cmd{
		&cli.VersionCmd{},
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
	err := cmd.Parse(os.Args[2:])
	if err != nil {
		log.Fatalf("[✗] %s", err)
	}

	// Finally run the command
	err = cmd.Run()
	if err != nil {
		log.Fatalf("[✗] %s", err)
	}
}
