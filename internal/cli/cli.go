package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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
		fmt.Fprintf(os.Stderr, "[✗] could not parse the template: %s\n", err)
		os.Exit(1)
	}
	err = tpl.Execute(w, data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[✗] could not execute the template: %s\n", err)
		os.Exit(1)
	}
}

func askPasswordWithConfirm() (string, error) {
	for {
		password, err := askPassword("Password")
		if err != nil {
			return "", err
		}
		confirm, err := askPassword("Confirm password")
		if err != nil {
			return "", err
		}
		if password == confirm {
			return password, nil
		}
		fmt.Println("[✗] Passwords do not match")
	}
}

func askPassword(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("standard input is not a terminal")
	}
	defer fmt.Println("")
	fmt.Printf("%s: ", prompt)
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

func askPasswordMode(prompt string, options []paw.PasswordMode, defaultMode paw.PasswordMode) (paw.PasswordMode, error) {
	var defaultIdx int
	for i, v := range options {
		if v == defaultMode {
			defaultIdx = i
			break
		}
	}
	fmt.Printf("%s [%s]:\n", prompt, options[defaultIdx])
	for i, v := range options {
		fmt.Printf("  [%d] %s\n", i, v)
	}
	for {
		fmt.Print("> ")
		choice, err := readLine()
		if err != nil {
			continue
		}
		if choice == "" {
			return options[defaultIdx], nil
		}
		idx, err := strconv.Atoi(choice)
		if err != nil {
			continue
		}
		if idx >= 0 && idx < len(options) {
			return options[idx], nil
		}
	}
}

func askIntWithDefaultAndRange(prompt string, def int, min int, max int) (int, error) {
	for {
		fmt.Printf("%s [%d]: ", prompt, def)
		v, err := readLine()
		if err != nil {
			continue
		}
		if v == "" {
			return def, nil
		}
		i, err := strconv.Atoi(v)
		if err != nil {
			continue
		}
		if i >= min && i <= max {
			return i, nil
		}
	}
}

func askYesNo(prompt string, defaultYes bool) (bool, error) {
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	for {
		fmt.Printf("%s [%s]: ", prompt, def)
		v, err := readLine()
		if err != nil {
			continue
		}
		if v == "" {
			return defaultYes, nil
		}
		switch strings.ToLower(v) {
		case "y":
			return true, nil
		case "n":
			return false, nil
		}
	}
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

type itemPath struct {
	itemName  string
	itemType  paw.ItemType
	vaultName string
}

func (i itemPath) String() string {
	return filepath.Join(i.vaultName, i.itemType.String(), i.itemName)
}

type itemPathOptions struct {
	fullPath bool
	wildcard bool
}

func parseItemPath(path string, opts itemPathOptions) (itemPath, error) {
	parts := strings.Split(path, "/")
	ip := itemPath{}

	if len(parts) > 3 || (opts.fullPath && len(parts) != 3) {
		return ip, fmt.Errorf("invalid vault item path. Got %q, expected VAULT_NAME/ITEM_TYPE/ITEM_NAME", path)
	}

	for i, v := range parts {
		if opts.fullPath && v == "" {
			return ip, fmt.Errorf("a path element is empty. Got %s, expected VAULT_NAME/ITEM_TYPE/ITEM_NAME", path)
		}
		switch i {
		case 0:
			ip.vaultName = v
		case 1:
			var itemType paw.ItemType
			var err error
			if opts.wildcard && v == "*" {
				break
			}
			itemType, err = paw.ItemTypeFromString(v)
			if err != nil {
				return ip, err
			}
			ip.itemType = itemType
		case 2:
			if opts.fullPath && v == "" {
				return ip, fmt.Errorf("item name cannot be empty")
			}
			ip.itemName = v
		}
	}
	return ip, nil
}
