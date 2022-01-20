package cli

import (
	"log"
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

	itemPath, err := parseItemPath(args[0], itemPathOptions{fullPath: true})
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
		log.Printf("URL: %s", v.URL)
		log.Printf("Username: %s", v.Username)
		log.Printf("Password: %s", v.Password.Value)
		if v.Note != nil {
			log.Printf("Note: %s", v.Note.Value)
		}
	case paw.PasswordItemType:
		v := item.(*paw.Password)
		log.Printf("Password: %s", v.Value)
		if v.Note != nil {
			log.Printf("Note: %s", v.Note.Value)
		}
	case paw.NoteItemType:
		v := item.(*paw.Note)
		log.Printf("Note: %s", v.Value)
	}

	log.Printf("Created: %s", item.GetMetadata().Created.Format(time.RFC1123))
	log.Printf("Modified: %s", item.GetMetadata().Modified.Format(time.RFC1123))
	return nil
}
