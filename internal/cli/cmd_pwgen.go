// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/paw"
)

// PwGenCmd implements the command that generates a password.
type PwGenCmd struct{ *command }

// newPwGenCommand constructs the password-generation command.
func newPwGenCommand(base *command) *urfave.Command {
	cmd := &PwGenCmd{command: base}
	c := &urfave.Command{
		Name:      "pwgen",
		Usage:     "Generates a password",
		ArgsUsage: " ",
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks that password generation has no positional arguments.
func (cmd *PwGenCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 0); err != nil {
		return ctx, err
	}
	return ctx, nil
}

// Run generates a password and writes it to the command output.
func (cmd *PwGenCmd) Run(ctx context.Context, _ *urfave.Command) error {
	modes := []paw.PasswordMode{
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	}
	password, err := cmd.Pwgen(modes, paw.RandomPassword)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, password.Value)
	return nil
}

// Pwgen prompts for a password mode and generates a password using that mode.
func (cmd *PwGenCmd) Pwgen(modes []paw.PasswordMode, defaultMode paw.PasswordMode) (*paw.PasswordSpec, error) {
	choice, err := cmd.prompter.askPasswordMode("Password type", modes, defaultMode)
	if err != nil {
		return nil, err
	}

	switch choice {
	case paw.CustomPassword:
		return cmd.makeCustomPassword()
	case paw.RandomPassword:
		return cmd.makeRandomPassword()
	case paw.PassphrasePassword:
		return cmd.makePassphrasePassword()
	case paw.PinPassword:
		return cmd.makePinPassword()
	}
	return nil, fmt.Errorf("unsupported password type: %q", choice)
}

// makeRandomPassword prompts for the length and symbol options and generates a password.
func (cmd *PwGenCmd) makeRandomPassword() (*paw.PasswordSpec, error) {
	p := paw.NewRandomPassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Password length", p.Length, 6, 64)
	if err != nil {
		return nil, err
	}
	p.Length = length
	p.Format = paw.UppercaseFormat | paw.LowercaseFormat | paw.DigitsFormat

	wantSymbols, err := cmd.prompter.askYesNo("Password should contains symbols?", true)
	if err != nil {
		return nil, err
	}
	if wantSymbols {
		p.Format |= paw.SymbolsFormat
	}
	v, err := p.Pwgen()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

// makeCustomPassword prompts for and confirms a custom password.
func (cmd *PwGenCmd) makeCustomPassword() (*paw.PasswordSpec, error) {
	p := paw.NewCustomPassword()
	v, err := cmd.prompter.askPasswordWithConfirm()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

// makePassphrasePassword prompts for the number of words and generates a passphrase.
func (cmd *PwGenCmd) makePassphrasePassword() (*paw.PasswordSpec, error) {
	p := paw.NewPassphrasePassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Passphrase words", p.Length, 2, 12)
	if err != nil {
		return nil, err
	}
	p.Length = length
	v, err := p.Pwgen()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

// makePinPassword prompts for a PIN length and generates a numeric PIN.
func (cmd *PwGenCmd) makePinPassword() (*paw.PasswordSpec, error) {
	p := paw.NewPinPassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Pin length", p.Length, 4, 10)
	if err != nil {
		return nil, err
	}
	p.Length = length
	p.Format = paw.DigitsFormat
	v, err := p.Pwgen()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}
