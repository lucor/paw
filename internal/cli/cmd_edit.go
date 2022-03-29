package cli

import (
	"fmt"
	"os"
	"time"

	"lucor.dev/paw/internal/paw"
)

// Edit edits an item into the vault
type EditCmd struct {
	itemPath
}

// Name returns the one word command name
func (cmd *EditCmd) Name() string {
	return "edit"
}

// Description returns the command description
func (cmd *EditCmd) Description() string {
	return "Edits an item into the vault"
}

// Usage displays the command usage
func (cmd *EditCmd) Usage() {
	template := `Usage: paw-cli [OPTION] edit VAULT_NAME/ITEM_TYPE/ITEM_NAME

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *EditCmd) Parse(args []string) error {
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
		cmd.editLoginItem(vault.Key(), item)
	case paw.NoteItemType:
		cmd.editNoteItem(item)
	case paw.PasswordItemType:
		cmd.editPasswordItem(vault.Key(), item)
	case paw.SSHKeyItemType:
		cmd.editSSHKeyItem(item)
	default:
		return fmt.Errorf("unsupported item type: %q", cmd.itemType)
	}

	item.GetMetadata().Modified = time.Now()

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
	fmt.Printf("[✓] item %q modified\n", cmd.itemName)
	return nil
}

func (cmd *EditCmd) editLoginItem(key *paw.Key, item paw.Item) error {
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

func (cmd *EditCmd) editPasswordItem(key *paw.Key, item paw.Item) error {
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

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}

func (cmd *EditCmd) editSSHKeyItem(item paw.Item) error {
	v := item.(*paw.SSHKey)

	note, err := askWithDefault("Note", v.Note.Value)
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}
