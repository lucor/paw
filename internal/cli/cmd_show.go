package cli

import (
	"fmt"
	"os"
	"time"

	"lucor.dev/paw/internal/paw"
)

// Show shows an item details
type ShowCmd struct {
	itemPath
}

// Name returns the one word command name
func (cmd *ShowCmd) Name() string {
	return "show"
}

// Description returns the command description
func (cmd *ShowCmd) Description() string {
	return "Shows an item details"
}

// Usage displays the command usage
func (cmd *ShowCmd) Usage() {
	template := `Usage: paw-cli show [OPTION] VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *ShowCmd) Parse(args []string) error {
	flags, err := newCommonFlags()
	if err != nil {
		return err
	}

	flagSet.Parse(args)
	if flags.Help || len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(0)
	}

	itemPath, err := parseItemPath(flagSet.Arg(0), itemPathOptions{fullPath: true})
	if err != nil {
		return err
	}
	cmd.itemPath = itemPath
	return nil
}

// Run runs the command
func (cmd *ShowCmd) Run(s paw.Storage) error {
	password, err := askPassword("Enter the vault password")
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, password)
	if err != nil {
		return err
	}

	item, err := paw.NewItem(cmd.itemName, cmd.itemType)
	if err != nil {
		return err
	}

	item, err = s.LoadItem(vault, item.GetMetadata())
	if err != nil {
		return err
	}

	switch cmd.itemType {
	case paw.LoginItemType:
		v := item.(*paw.Login)
		fmt.Printf("URL: %s\n", v.URL)
		fmt.Printf("Username: %s\n", v.Username)
		fmt.Printf("Password: %s\n", v.Password.Value)
		if v.Note != nil {
			fmt.Printf("Note: %s\n", v.Note.Value)
		}
	case paw.PasswordItemType:
		v := item.(*paw.Password)
		fmt.Printf("Password: %s\n", v.Value)
		if v.Note != nil {
			fmt.Printf("Note: %s\n", v.Note.Value)
		}
	case paw.NoteItemType:
		v := item.(*paw.Note)
		fmt.Printf("Note: %s\n", v.Value)
	}

	fmt.Printf("Created: %s\n", item.GetMetadata().Created.Format(time.RFC1123))
	fmt.Printf("Modified: %s\n", item.GetMetadata().Modified.Format(time.RFC1123))
	return nil
}
