// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"lucor.dev/paw/internal/paw"
	"lucor.dev/paw/internal/tree"
)

// List lists the vaults content
type ListCmd struct {
	*command
	sessionID string
	itemPath
}

func newListCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "ls",
		Usage:     "Lists vaults and items",
		ArgsUsage: "[VAULT_NAME/ITEM_TYPE/ITEM_NAME]",
		Flags:     []urfave.Flag{sessionFlag()},
		Action: withArgs(0, 1, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			cmd := &ListCmd{command: base, sessionID: c.String("session")}
			if c.Args().Present() {
				path, err := parseItemPath(c.Args().First(), itemPathOptions{wildcard: true})
				if err != nil {
					return err
				}
				cmd.itemPath = path
			}
			return cmd.Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *ListCmd) Run(ctx context.Context) error {
	vaultNode, err := cmd.vaults()
	if err != nil {
		return err
	}

	if cmd.vaultName == "" {
		return tree.Fprint(cmd.out, vaultNode)
	}

	itemsNode, err := cmd.items()
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
		fmt.Fprintf(cmd.out, "vault %q is empty\n", cmd.vaultName)
		return nil
	}

	return tree.Fprint(cmd.out, n)
}

func (cmd *ListCmd) items() ([]tree.Node, error) {
	s := cmd.storage
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
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

func (cmd *ListCmd) vaults() (tree.Node, error) {
	s := cmd.storage

	n := tree.Node{
		Value: "paw",
	}
	vaults, err := s.Vaults()
	if err != nil {
		return n, err
	}
	if len(vaults) == 0 {
		return n, fmt.Errorf("no vaults found. To create one: paw cli init VAULT")
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
