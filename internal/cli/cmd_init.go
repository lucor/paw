package cli

import (
	"fmt"
	"log"

	"lucor.dev/paw/internal/paw"
)

// Init initializes a vault
type InitCmd struct {
	vaultName string
}

// Name returns the one word command name
func (cmd *InitCmd) Name() string {
	return "init"
}

// Description returns the command description
func (cmd *InitCmd) Description() string {
	return "Initializes a vault"
}

// Run runs the command
func (cmd *InitCmd) Run(s paw.Storage) error {
	fmt.Printf("Initializing vault %q\n", cmd.vaultName)
	password, err := askPassword("Enter the vault password")
	if err != nil {
		return err
	}
	key, err := s.CreateVaultKey(cmd.vaultName, password)
	if err != nil {
		return err
	}

	_, err = s.CreateVault(cmd.vaultName, key)
	if err != nil {
		return err
	}
	log.Printf("[✓] vault %q created", cmd.vaultName)
	return nil
}

// Parse parses the arguments and set the usage for the command
func (cmd *InitCmd) Parse(args []string) error {
	if len(args) == 0 {
		return nil
	}

	cmd.vaultName = args[0]
	return nil
}

// Usage displays the command usage
func (cmd *InitCmd) Usage() {
	template := `Usage: paw-cli init VAULT

{{ . }}
`
	printUsage(template, cmd.Description())
}
