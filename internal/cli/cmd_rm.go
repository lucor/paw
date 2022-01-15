package cli

import (
	"fmt"
	"log"
	"strings"

	"lucor.dev/paw/internal/paw"
)

// RemoveCmd removes an item from the vault
type RemoveCmd struct {
	itemName  string
	itemType  paw.ItemType
	vaultName string
}

// Name returns the one word command name
func (cmd *RemoveCmd) Name() string {
	return "rm"
}

// Description returns the command description
func (cmd *RemoveCmd) Description() string {
	return "Removes an item from the vault"
}

// Run runs the command
func (cmd *RemoveCmd) Run(s paw.Storage) error {
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

	if ok := vault.HasItem(item); !ok {
		return fmt.Errorf("item does not exists")
	}

	err = s.DeleteItem(vault, item)
	if err != nil {
		return err
	}

	vault.DeleteItem(item)

	err = s.StoreVault(vault)
	if err != nil {
		return err
	}

	log.Printf("[✓] item %q removed", cmd.itemName)
	return nil
}

// Parse parses the arguments and set the usage for the command
func (cmd *RemoveCmd) Parse(args []string) error {
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
func (cmd *RemoveCmd) Usage() {
	template := `Usage: paw-cli add VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}
`
	printUsage(template, cmd.Description())
}

func (cmd *RemoveCmd) addLoginItem(item paw.Item) error {
	v := item.(*paw.Login)

	url, err := ask("URL")
	if err != nil {
		return err
	}
	v.URL = url

	username, err := ask("Username")
	if err != nil {
		return err
	}
	v.Username = username

	password, err := ask("Password")
	if err != nil {
		return err
	}
	v.Password.Value = password

	note, err := ask("Note")
	if err != nil {
		return err
	}
	v.Note.Value = note

	item = v
	return nil
}

func (cmd *RemoveCmd) addNoteItem(item paw.Item) error {
	v := item.(*paw.Note)

	note, err := ask("Note")
	if err != nil {
		return err
	}
	v.Value = note

	item = v
	return nil
}

func (cmd *RemoveCmd) addPasswordItem(item paw.Item) error {
	v := item.(*paw.Password)

	password, err := ask("Password")
	if err != nil {
		return err
	}
	v.Value = password

	note, err := ask("Note")
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}
