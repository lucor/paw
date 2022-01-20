package cli

import (
	"fmt"
	"log"
	"os"

	"lucor.dev/paw/internal/paw"
)

// Add adds an item to the vault
type AddCmd struct {
	itemPath
}

// Name returns the one word command name
func (cmd *AddCmd) Name() string {
	return "add"
}

// Description returns the command description
func (cmd *AddCmd) Description() string {
	return "Adds an item to the vault"
}

// Usage displays the command usage
func (cmd *AddCmd) Usage() {
	template := `Usage: paw-cli add [OPTION] VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *AddCmd) Parse(args []string) error {
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
func (cmd *AddCmd) Run(s paw.Storage) error {
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

	if ok := vault.HasItem(item); ok {
		return fmt.Errorf("item with same name already exists")
	}

	switch cmd.itemType {
	case paw.LoginItemType:
		cmd.addLoginItem(vault.Key(), item)
	case paw.NoteItemType:
		cmd.addNoteItem(item)
	case paw.PasswordItemType:
		cmd.addPasswordItem(vault.Key(), item)
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
	log.Printf("[✓] item %q added", cmd.itemName)
	return nil
}

func (cmd *AddCmd) addLoginItem(key *paw.Key, item paw.Item) error {
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

	pwgenCmd := &PwGenCmd{}
	modes := []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	}
	password, err := pwgenCmd.Pwgen(key, modes, v.Mode)
	if err != nil {
		return err
	}
	v.Password.Value = password.Value
	v.Password.Mode = password.Mode
	v.Password.Format = password.Format
	v.Password.Length = password.Length

	note, err := ask("Note")
	if err != nil {
		return err
	}
	v.Note.Value = note

	item = v
	return nil
}

func (cmd *AddCmd) addNoteItem(item paw.Item) error {
	v := item.(*paw.Note)

	note, err := ask("Note")
	if err != nil {
		return err
	}
	v.Value = note

	item = v
	return nil
}

func (cmd *AddCmd) addPasswordItem(key *paw.Key, item paw.Item) error {
	v := item.(*paw.Password)

	pwgenCmd := &PwGenCmd{}
	modes := []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	}
	password, err := pwgenCmd.Pwgen(key, modes, v.Mode)
	if err != nil {
		return err
	}
	v.Value = password.Value
	v.Mode = password.Mode
	v.Format = password.Format
	v.Length = password.Length

	note, err := ask("Note")
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}
