package cli

import (
	"fmt"
	"log"
	"strings"

	"lucor.dev/paw/internal/paw"
)

// Edit edits an item into the vault
type EditCmd struct {
	itemName  string
	itemType  paw.ItemType
	vaultName string
}

// Name returns the one word command name
func (cmd *EditCmd) Name() string {
	return "edit"
}

// Description returns the command description
func (cmd *EditCmd) Description() string {
	return "Edits an item into the vault"
}

// Run runs the command
func (cmd *EditCmd) Run(s paw.Storage) error {
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
		cmd.editLoginItem(item)
	case paw.NoteItemType:
		cmd.editNoteItem(item)
	case paw.PasswordItemType:
		cmd.editPasswordItem(item)
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}

	err = s.StoreItem(vault, item)
	if err != nil {
		return err
	}
	err = vault.AddItem(item)
	if err != nil {
		return err
	}
	err = s.StoreVault(vault)
	if err != nil {
		return err
	}
	log.Printf("[✓] item %q modified", cmd.itemName)
	return nil
}

// Parse parses the arguments and set the usage for the command
func (cmd *EditCmd) Parse(args []string) error {
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
func (cmd *EditCmd) Usage() {
	template := `Usage: paw-cli add VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}
`
	printUsage(template, cmd.Description())
}

func (cmd *EditCmd) editLoginItem(item paw.Item) error {
	v := item.(*paw.Login)

	url, err := askWithDefault("URL", v.URL)
	if err != nil {
		return err
	}
	v.URL = url

	username, err := askWithDefault("Username", v.Username)
	if err != nil {
		return err
	}
	v.Username = username

	password, err := askWithDefault("Password", v.Password.Value)
	if err != nil {
		return err
	}
	v.Password.Value = password

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}
	v.Note.Value = note

	item = v
	return nil
}

func (cmd *EditCmd) editNoteItem(item paw.Item) error {
	v := item.(*paw.Note)

	note, err := askWithDefault("Note", v.Value)
	if err != nil {
		return err
	}
	v.Value = note

	item = v
	return nil
}

func (cmd *EditCmd) editPasswordItem(item paw.Item) error {
	v := item.(*paw.Password)

	password, err := askWithDefault("Password", v.Value)
	if err != nil {
		return err
	}
	v.Value = password

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}
