package cli

import (
	"fmt"
	"log"
	"strings"
	"time"

	"lucor.dev/paw/internal/paw"
)

// Show shows an item details
type ShowCmd struct {
	itemName  string
	itemType  paw.ItemType
	vaultName string
}

// Name returns the one word command name
func (cmd *ShowCmd) Name() string {
	return "show"
}

// Description returns the command description
func (cmd *ShowCmd) Description() string {
	return "Shows an item details"
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

	item, err := paw.NewItemFromType(cmd.itemType)
	if err != nil {
		return err
	}
	item.GetMetadata().Name = cmd.itemName

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

// Parse parses the arguments and set the usage for the command
func (cmd *ShowCmd) Parse(args []string) error {
	if len(args) == 0 {
		return nil
	}

	parts := strings.Split(args[0], "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid path. Got %s, expected VAULT_NAME/ITEM_TYPE/ITEM_NAME", args[0])
	}

	for i, v := range parts {
		switch i {
		case 0:
			if v == "" {
				return fmt.Errorf("vault name cannot be empty")
			}
			cmd.vaultName = v
		case 1:
			if v == "" {
				return fmt.Errorf("item type cannot be empty")
			}
			itemType, err := paw.ItemTypeFromString(v)
			if err != nil {
				return err
			}
			cmd.itemType = itemType
		case 2:
			if v == "" {
				return fmt.Errorf("item name cannot be empty")
			}
			cmd.itemName = v
		}
	}
	return nil
}

// Usage displays the command usage
func (cmd *ShowCmd) Usage() {
	template := `Usage: paw-cli show VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}
`
	printUsage(template, cmd.Description())
}
