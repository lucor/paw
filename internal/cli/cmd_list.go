package cli

import (
	"fmt"
	"strings"

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/tree"
)

// List is the version command
type ListCmd struct {
	vault    string
	itemType paw.ItemType
	itemName string
}

// Name returns the one word command name
func (cmd *ListCmd) Name() string {
	return "ls"
}

// Description returns the command description
func (cmd *ListCmd) Description() string {
	return "List vaults and items"
}

// Run runs the command
func (cmd *ListCmd) Run(s paw.Storage) error {
	vaultNode, err := cmd.vaults(s)
	if err != nil {
		return err
	}
	if cmd.vault == "" {
		tree.Print(vaultNode)
		return nil
	}

	itemsNode, err := cmd.items(s)
	if err != nil {
		return err
	}

	n := tree.Node{Value: "paw/" + cmd.vault}
	for _, v := range itemsNode {
		if len(v.Child) == 0 {
			continue
		}
		n.Child = append(n.Child, v)
	}
	tree.Print(n)
	return nil
}

// Parse parses the arguments and set the usage for the command
func (cmd *ListCmd) Parse(args []string) error {
	if len(args) == 0 {
		return nil
	}

	parts := strings.Split(args[0], "/")
	if len(parts) > 3 {
		return fmt.Errorf("invalid path: %s", args[0])
	}
	for i, v := range parts {
		switch i {
		case 0:
			cmd.vault = v
		case 1:
			var itemType paw.ItemType
			switch v {
			case paw.LoginItemType.String():
				itemType = paw.LoginItemType
			case paw.NoteItemType.String():
				itemType = paw.NoteItemType
			case paw.PasswordItemType.String():
				itemType = paw.PasswordItemType
			}

			if itemType == 0 && v != "" && v != "*" {
				return fmt.Errorf("invalid Paw item type %q", v)
			}
			cmd.itemType = itemType
		case 2:
			cmd.itemName = v
		}
	}
	return nil
}

// Usage displays the command usage
func (cmd *ListCmd) Usage() {
	template := `Usage: paw-cli ls

{{ . }}
`
	printUsage(template, cmd.Description())
}

func (cmd *ListCmd) items(s paw.Storage) ([]tree.Node, error) {
	password, err := readPassword("Enter the vault password:")
	if err != nil {
		return nil, err
	}
	vault, err := s.LoadVault(cmd.vault, password)
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
	for _, v := range meta {
		switch v.Type {
		case paw.LoginItemType:
			loginNode.Child = append(loginNode.Child, tree.Node{Value: v.Name})
		case paw.NoteItemType:
			noteNode.Child = append(noteNode.Child, tree.Node{Value: v.Name})
		case paw.PasswordItemType:
			passwordNode.Child = append(passwordNode.Child, tree.Node{Value: v.Name})
		}
	}

	return []tree.Node{
		loginNode,
		noteNode,
		passwordNode,
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
	for _, v := range vaults {
		if cmd.vault != "" && cmd.vault != v {
			continue
		}
		n.Child = append(n.Child, tree.Node{Value: v})
	}
	if len(n.Child) == 0 {
		return n, fmt.Errorf("vault %q does not exists", cmd.vault)
	}
	return n, nil
}
