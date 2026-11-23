// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	urfave "github.com/urfave/cli/v3"
	"go.lucor.dev/paw/internal/model"
)

// completionArgValidator runs command validation only during normal execution.
func completionArgValidator(validate urfave.BeforeFunc) urfave.ArgValidatorFunc {
	return func(ctx context.Context, c *urfave.Command) error {
		_, err := validate(ctx, c)
		return err
	}
}

// completeVaults suggests vault names or the beginnings of item paths.
func (base *command) completeVaults(itemPath bool) urfave.ShellCompleteFunc {
	return func(ctx context.Context, c *urfave.Command) {
		args := c.Args().Slice()
		if len(args) > 0 && strings.HasPrefix(args[len(args)-1], "-") {
			urfave.DefaultCompleteWithFlags(ctx, c)
			return
		}
		if len(args) != 0 || base.storage == nil {
			return
		}
		vaults, err := base.storage.Vaults()
		if err != nil {
			return
		}
		for _, vault := range vaults {
			if vault.Name == "" || strings.HasPrefix(vault.Name, "-") ||
				strings.ContainsFunc(vault.Name, func(r rune) bool {
					return !unicode.IsLetter(r) && !unicode.IsNumber(r) && !strings.ContainsRune(" _-.", r)
				}) {
				continue
			}
			out := c.Root().Writer
			if !itemPath {
				fmt.Fprintln(out, vault.Name)
				continue
			}
			for _, kind := range []model.ItemType{model.LoginItemType, model.PasswordItemType, model.NoteItemType, model.SSHKeyItemType} {
				fmt.Fprintf(out, "%s/%s/\n", vault.Name, kind)
			}
		}
	}
}
