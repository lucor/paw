// SPDX-FileCopyrightText: 2022-2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// newAddPasswordCommand registers add password.
func newAddPasswordCommand(base *command) *urfave.Command {
	cmd := &AddCmd{command: base, itemPath: itemPath{itemType: model.PasswordItemType}}
	return &urfave.Command{
		Name:         "password",
		Usage:        "Adds a " + model.PasswordItemType.Label() + " item",
		ArgsUsage:    "VAULT NAME",
		Description:  vaultReferenceHelp,
		Before:       cmd.validatePassword,
		Action:       cmd.Run,
		Flags:        addPasswordFlags(),
		OnUsageError: onUsageError,
	}
}

// addPasswordFlags returns the password flags shared by add login and add password.
func addPasswordFlags() []urfave.Flag {
	return []urfave.Flag{
		&urfave.BoolFlag{Name: "generate-password", Usage: "Generate a random password with default settings"},
		&urfave.BoolFlag{Name: "password-stdin", Usage: "Read the password from stdin"},
	}
}

// validatePassword checks the arguments and the selected password source.
func (cmd *AddCmd) validatePassword(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := cmd.validateItem(c); err != nil {
		return ctx, err
	}
	return ctx, cmd.validatePasswordFlags(c)
}

// validatePasswordFlags checks that only one password source is selected.
func (cmd *AddCmd) validatePasswordFlags(c *urfave.Command) error {
	cmd.randomPassword = c.Bool("generate-password")
	cmd.passwordStdin = c.Bool("password-stdin")
	if cmd.randomPassword && cmd.passwordStdin {
		return invalidArgumentsError("--generate-password and --password-stdin cannot be combined")
	}
	if cmd.passwordStdin {
		cmd.input = c.Reader
	}
	return nil
}

// newPassword creates an item with the supplied or generated password.
func (cmd *AddCmd) newPassword() (*model.Item, error) {
	password, err := cmd.generatePassword(paw.CustomPassword)
	if err != nil {
		return nil, err
	}
	return model.NewPassword(cmd.itemName, password.Value, model.PasswordSettings{
		Mode:   password.Mode,
		Length: password.Length,
		Format: password.Format,
	})
}

// generatePassword uses the explicit source or prompts for a password mode.
func (cmd *AddCmd) generatePassword(defaultMode paw.PasswordMode) (*paw.PasswordSpec, error) {
	if cmd.randomPassword {
		password := paw.NewRandomPassword()
		var err error
		password.Value, err = password.Pwgen()
		return password, err
	}
	if cmd.passwordStdin {
		password := paw.NewCustomPassword()
		var err error
		password.Value, err = readAddSecret(cmd.input)
		if err != nil {
			return nil, fmt.Errorf("read password from stdin: %w", err)
		}
		return password, nil
	}
	return (&PwGenCmd{command: cmd.command}).Pwgen([]paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword}, defaultMode)
}

// readAddSecret reads one nonempty line, preserving spaces in the secret.
func readAddSecret(input io.Reader) (string, error) {
	data, err := io.ReadAll(input)
	if err != nil {
		return "", err
	}
	value := strings.TrimSuffix(string(data), "\n")
	value = strings.TrimSuffix(value, "\r")
	if value == "" || strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("expected one nonempty line")
	}
	return value, nil
}
