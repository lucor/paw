// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"fmt"

	urfave "github.com/urfave/cli/v3"

	"go.lucor.dev/paw/internal/paw"
)

// PwGenCmd generates a password
type PwGenCmd struct{ *command }

func newPwGenCommand(base *command) *urfave.Command {
	return &urfave.Command{
		Name:      "pwgen",
		Usage:     "Generates a password",
		ArgsUsage: " ",
		Action: withArgs(0, 0, func(ctx context.Context, c *urfave.Command) error {
			base := base.forInvocation(c)
			return (&PwGenCmd{command: base}).Run(ctx)
		}),
	}
}

// Run runs the command
func (cmd *PwGenCmd) Run(ctx context.Context) error {
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

// Parse parses the arguments and set the usage for the command
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

func (cmd *PwGenCmd) makeCustomPassword() (*paw.PasswordSpec, error) {
	p := paw.NewCustomPassword()
	v, err := cmd.prompter.askPasswordWithConfirm()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

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
