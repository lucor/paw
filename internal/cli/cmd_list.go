package cli

import (
	"fmt"

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/tree"
)

// List lists the vaults content
type ListCmd struct {
	itemPath
}

// Name returns the one word command name
func (cmd *ListCmd) Name() string {
	return "ls"
}

// Description returns the command description
func (cmd *ListCmd) Description() string {
	return "List vaults and items"
}

// Usage displays the command usage
func (cmd *ListCmd) Usage() {
	template := `Usage: paw-cli ls [OPTION] [VAULT_NAME/ITEM_TYPE/ITEM_NAME]

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *ListCmd) Parse(args []string) error {
	flags, err := newCommonFlags(flagOpts{Session: true})
	if err != nil {
		return err
	}

	flags.Parse(cmd, args)
	flags.SetEnv()
	if len(flagSet.Args()) == 0 {
		return nil
	}

	itemPath, err := parseItemPath(flagSet.Arg(0), itemPathOptions{wildcard: true})
	if err != nil {
		return err
	}

	cmd.itemPath = itemPath
	return nil
}

// Run runs the command
func (cmd *ListCmd) Run(s paw.Storage) error {
	vaultNode, err := cmd.vaults(s)
	if err != nil {
		return err
	}

	if cmd.vaultName == "" {
		tree.Print(vaultNode)
		return nil
	}

	itemsNode, err := cmd.items(s)
	if err != nil {
		return err
	}

	n := tree.Node{Value: "paw/" + cmd.vaultName}
	for _, v := range itemsNode {
		if len(v.Child) == 0 {
			continue
		}
		n.Child = append(n.Child, v)
	}

	if len(n.Child) == 0 {
		fmt.Printf("vault %q is empty\n", cmd.vaultName)
		return nil
	}

	tree.Print(n)
	return nil
}

func (cmd *ListCmd) items(s paw.Storage) ([]tree.Node, error) {
	key, err := loadVaultKey(s, cmd.vaultName)
	if err != nil {
		return nil, err
	}

	vault, err := s.LoadVault(cmd.vaultName, key)
	if err != nil {
		return nil, err
	}

	meta := vault.FilterItemMetadata(&paw.VaultFilterOptions{
		Name:     cmd.itemName,
		ItemType: cmd.itemType,
	})

	loginNode := tree.Node{Value: paw.LoginItemType.String()}
	noteNode := tree.Node{Value: paw.NoteItemType.String()}
	passwordNode := tree.Node{Value: paw.PasswordItemType.String()}
	sshkeyNode := tree.Node{Value: paw.SSHKeyItemType.String()}
	for _, v := range meta {
		switch v.Type {
		case paw.LoginItemType:
			loginNode.Child = append(loginNode.Child, tree.Node{Value: v.Name})
		case paw.NoteItemType:
			noteNode.Child = append(noteNode.Child, tree.Node{Value: v.Name})
		case paw.PasswordItemType:
			passwordNode.Child = append(passwordNode.Child, tree.Node{Value: v.Name})
		case paw.SSHKeyItemType:
			sshkeyNode.Child = append(sshkeyNode.Child, tree.Node{Value: v.Name})
		}
	}

	return []tree.Node{
		loginNode,
		noteNode,
		passwordNode,
		sshkeyNode,
	}, nil
}

func (cmd *ListCmd) vaults(s paw.Storage) (tree.Node, error) {

	n := tree.Node{
		Value: "paw",
	}
	vaults, err := s.Vaults()
	if err != nil {
		return n, err
	}
	if len(vaults) == 0 {
		return n, fmt.Errorf("no vaults found. To create one: paw-cli init VAULT")
	}
	for _, v := range vaults {
		if cmd.vaultName != "" && cmd.vaultName != v {
			continue
		}
		n.Child = append(n.Child, tree.Node{Value: v})
	}
	if len(n.Child) == 0 {
		return n, fmt.Errorf("vault %q does not exists", cmd.vaultName)
	}
	return n, nil
}
