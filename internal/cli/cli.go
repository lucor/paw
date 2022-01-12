package cli

import (
	"io"
	"log"
	"os"
	"text/template"
)

// Cmd wraps the methods for a paw-cli command
type Cmd interface {
	Name() string              // Name returns the one word command name
	Description() string       // Description returns the command description
	Parse(args []string) error // Parse parses the cli arguments
	Usage()                    // Usage displays the command usage
	Run() error                // Run runs the command
}

// Usage prints the command usage
func Usage(commands []Cmd) {
	template := `paw-cli is the CLI application for Paw

Usage: paw-cli <command> [arguments]

The commands are:

{{ range $k, $cmd := . }}	{{ printf "%-13s %s\n" $cmd.Name $cmd.Description }}{{ end }}
Use "paw-cli <command> -help" for more information about a command.
`
	printTemplate(os.Stdout, template, commands)
}

func printUsage(textTemplate string, data interface{}) {
	printTemplate(os.Stdout, textTemplate, data)
}

// PrintTemplate prints the parsed text template to the specified data object,
// and writes the output to w.
func printTemplate(w io.Writer, textTemplate string, data interface{}) {
	tpl, err := template.New("tpl").Parse(textTemplate)
	if err != nil {
		log.Fatalf("Could not parse the template: %s", err)
	}
	err = tpl.Execute(w, data)
	if err != nil {
		log.Fatalf("Could not execute the template: %s", err)
	}
}
