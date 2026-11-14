// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/tree"
)

// List lists the vaults content
type ListCmd struct {
	*command
	sessionID string
	itemPath
}

// newListCommand constructs the vault and item listing command.
func newListCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:        "ls",
		Usage:       "Lists vaults and items",
		ArgsUsage:   "[VAULT/ITEM_TYPE/ITEM_NAME]",
		Description: vaultReferenceHelp,
		Flags:       []urfave.Flag{sessionFlag()},
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

// Run lists the selected vaults or their items.
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

	n := tree.Node{Value: vaultNode.Child[0].Value}
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

// items builds the tree of items in the selected vault.
func (cmd *ListCmd) items() ([]tree.Node, error) {
	key, err := cmd.loadVaultKey(cmd.vaultName, cmd.sessionID)
	if err != nil {
		return nil, err
	}

	vault, err := cmd.loadVault(key)
	if err != nil {
		return nil, err
	}

	items, err := vault.FilterItems(cmd.itemName, cmd.itemType)
	if err != nil {
		return nil, err
	}

	loginNode := tree.Node{Value: model.LoginItemType.String()}
	noteNode := tree.Node{Value: model.NoteItemType.String()}
	passwordNode := tree.Node{Value: model.PasswordItemType.String()}
	sshkeyNode := tree.Node{Value: model.SSHKeyItemType.String()}
	for index := range items {
		v := &items[index]
		itemType, err := model.ItemTypeOf(v)
		if err != nil {
			return nil, err
		}
		switch itemType {
		case model.LoginItemType:
			loginNode.Child = append(loginNode.Child, tree.Node{Value: v.Title})
		case model.NoteItemType:
			noteNode.Child = append(noteNode.Child, tree.Node{Value: v.Title})
		case model.PasswordItemType:
			passwordNode.Child = append(passwordNode.Child, tree.Node{Value: v.Title})
		case model.SSHKeyItemType:
			sshkeyNode.Child = append(sshkeyNode.Child, tree.Node{Value: v.Title})
		}
	}

	return []tree.Node{
		loginNode,
		noteNode,
		passwordNode,
		sshkeyNode,
	}, nil
}

// vaults builds the tree of available vaults.
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
	ids := make([]string, len(vaults))
	for i := range vaults {
		ids[i] = vaults[i].ID
	}
	prefixes := vaultIDPrefixes(ids)
	var selected *model.VaultInfo
	if cmd.vaultName != "" {
		info, err := resolveVaultInfo(vaults, cmd.vaultName)
		if err != nil {
			return n, err
		}
		selected = &info
	}
	for _, v := range vaults {
		if selected != nil && selected.ID != v.ID {
			continue
		}
		n.Child = append(n.Child, tree.Node{Value: fmt.Sprintf("%s (%s)", v.Name, prefixes[v.ID])})
	}
	return n, nil
}
