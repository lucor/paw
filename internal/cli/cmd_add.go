package cli

import (
	"fmt"
	"os"

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/sshkey"
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
  -h, --help                  Displays this help and exit
      --session=SESSION_ID    Sets a session ID to use instead of the env var
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *AddCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{Session: true})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	if len(flagSet.Args()) != 1 {
		cmd.Usage()
		os.Exit(1)
	}
	flags.SetEnv()

	itemPath, err := parseItemPath(flagSet.Arg(0), itemPathOptions{fullPath: true})
	if err != nil {
		return err
	}
	cmd.itemPath = itemPath
	return nil
}

// Run runs the command
func (cmd *AddCmd) Run(s paw.Storage) error {
	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return err
	}

	vault, err := s.LoadVault(cmd.vaultName, key)
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
	case paw.SSHKeyItemType:
		cmd.addSSHKeyItem(item)
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
	fmt.Printf("[✓] item %q added\n", cmd.itemName)
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

func (cmd *AddCmd) addSSHKeyItem(item paw.Item) error {
	v := item.(*paw.SSHKey)

	k, err := sshkey.GenerateKey()
	if err != nil {
		return err
	}

	v.PrivateKey = string(k.MarshalPrivateKey())
	v.PublicKey = string(k.MarshalPublicKey())
	v.Fingerprint = k.Fingerprint()

	note, err := ask("Note")
	if err != nil {
		return err
	}

	v.Note.Value = note
	item = v
	return nil
}
