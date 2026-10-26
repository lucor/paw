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
	password, err := cmd.Pwgen(nil, modes, paw.RandomPassword)
	if err != nil {
		return err
	}

	fmt.Fprintln(cmd.out, password.Value)
	return nil
}

func (cmd *PwGenCmd) Pwgen(key *paw.Key, modes []paw.PasswordMode, defaultMode paw.PasswordMode) (*paw.Password, error) {

	var err error

	if key == nil {
		key, err = cmd.oneTimeKey()
		if err != nil {
			return nil, err
		}
	}

	choice, err := cmd.prompter.askPasswordMode("Password type", modes, defaultMode)
	if err != nil {
		return nil, err
	}

	switch choice {
	case paw.CustomPassword:
		return cmd.makeCustomPassword()
	case paw.RandomPassword:
		return cmd.makeRandomPassword(key)
	case paw.PassphrasePassword:
		return cmd.makePassphrasePassword(key)
	case paw.PinPassword:
		return cmd.makePinPassword(key)
	}
	return nil, fmt.Errorf("unsupported password type: %q", choice)
}

// Parse parses the arguments and set the usage for the command
func (cmd *PwGenCmd) makeRandomPassword(key *paw.Key) (*paw.Password, error) {
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
	v, err := key.Secret(p)
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makeCustomPassword() (*paw.Password, error) {
	p := paw.NewCustomPassword()
	v, err := cmd.prompter.askPasswordWithConfirm()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makePassphrasePassword(key *paw.Key) (*paw.Password, error) {
	p := paw.NewPassphrasePassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Passphrase words", p.Length, 2, 12)
	if err != nil {
		return nil, err
	}
	v, err := key.Passphrase(length)
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makePinPassword(key *paw.Key) (*paw.Password, error) {
	p := paw.NewPinPassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Pin length", p.Length, 4, 10)
	if err != nil {
		return nil, err
	}
	p.Length = length
	p.Format = paw.DigitsFormat
	v, err := key.Secret(p)
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}
