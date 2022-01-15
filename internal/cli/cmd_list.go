package cli

import (
	"fmt"
	"log"
	"strings"

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/tree"
)

// List lists the vaults content
type ListCmd struct {
	itemName  string
	itemType  paw.ItemType
	vaultName string
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
		log.Printf("vault %q is empty", cmd.vaultName)
		return nil
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
			cmd.vaultName = v
		case 1:
			var itemType paw.ItemType
			var err error
			if v != "*" && v != "" {
				itemType, err = paw.ItemTypeFromString(v)
				if err != nil {
					return err
				}
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
	password, err := askPassword("Enter the vault password")
	if err != nil {
		return nil, err
	}
	vault, err := s.LoadVault(cmd.vaultName, password)
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
