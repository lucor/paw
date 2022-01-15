package cli

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"text/template"

	"golang.org/x/term"
	"lucor.dev/paw/internal/paw"
)

// Cmd wraps the methods for a paw-cli command
type Cmd interface {
	Name() string              // Name returns the one word command name
	Description() string       // Description returns the command description
	Parse(args []string) error // Parse parses the cli arguments
	Usage()                    // Usage displays the command usage
	Run(s paw.Storage) error   // Run runs the command
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

func askPassword(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("standard input is not a terminal")
	}
	defer fmt.Println("")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", fmt.Errorf("could not read password from standard input: %w", err)
	}
	return string(password), nil
}

func askWithDefault(prompt string, old string) (string, error) {
	fmt.Printf("%s [%s]: ", prompt, old)
	line, err := readLine()
	if err != nil {
		return old, err
	}
	if line == "" {
		return old, err
	}
	return line, nil
}

func ask(prompt string) (string, error) {
	fmt.Printf("%s: ", prompt)
	return readLine()
}

func readLine() (string, error) {
	var out string
	scanner := bufio.NewScanner(bufio.NewReader(os.Stdin))
	for scanner.Scan() {
		out = scanner.Text()
		break
	}
	if err := scanner.Err(); err != nil {
		return out, fmt.Errorf("error reading standard input: %w", err)
	}
	return out, nil
}
