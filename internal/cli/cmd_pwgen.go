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
type PwGenCmd struct {
	*command
	clip bool
}

// newPwGenCommand constructs the password-generation command.
func newPwGenCommand(base *command) *urfave.Command {
	cmd := &PwGenCmd{command: base}
	c := &urfave.Command{
		Name:      "pwgen",
		Usage:     "Generates a password",
		ArgsUsage: " ",
		Flags: []urfave.Flag{
			&urfave.BoolFlag{
				Name:    "clip",
				Aliases: []string{"c"},
				Usage:   "Copy the generated password to the clipboard",
			},
		},
		OnUsageError: onUsageError,
		Commands: []*urfave.Command{
			newPwGenRandomCommand(cmd),
			newPwGenPassphraseCommand(cmd),
			newPwGenPinCommand(cmd),
		},
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks that password generation has no positional arguments.
func (cmd *PwGenCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	cmd.clip = c.Bool("clip")
	if c.Args().Present() {
		switch c.Args().First() {
		case "random", "passphrase", "pin":
			return ctx, nil
		}
	}
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

	return cmd.writePassword(ctx, password.Value)
}

// writePassword sends a generated password to the clipboard or standard output.
func (cmd *PwGenCmd) writePassword(ctx context.Context, password string) error {
	if cmd.clip {
		if err := cmd.command.copyValue(ctx, password); err != nil {
			return err
		}
		return cmd.Infof("Password copied to clipboard\n")
	}
	return cmd.Printf("%s\n", password)
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
	defaultPassword := paw.NewRandomPassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Password length", defaultPassword.Length, 6, 64)
	if err != nil {
		return nil, err
	}
	wantSymbols, err := cmd.prompter.askYesNo("Password should contains symbols?", true)
	if err != nil {
		return nil, err
	}
	return generateRandomPassword(length, wantSymbols)
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
	defaultPassword := paw.NewPassphrasePassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Passphrase words", defaultPassword.Length, 2, 12)
	if err != nil {
		return nil, err
	}
	return generatePassphrasePassword(length)
}

// makePinPassword prompts for a PIN length and generates a numeric PIN.
func (cmd *PwGenCmd) makePinPassword() (*paw.PasswordSpec, error) {
	defaultPassword := paw.NewPinPassword()
	length, err := cmd.prompter.askIntWithDefaultAndRange("Pin length", defaultPassword.Length, 4, 10)
	if err != nil {
		return nil, err
	}
	return generatePinPassword(length)
}

// generateRandomPassword creates a random password with the selected options.
func generateRandomPassword(length int, symbols bool) (*paw.PasswordSpec, error) {
	password := paw.NewRandomPassword()
	password.Length = length
	if !symbols {
		password.Format &^= paw.SymbolsFormat
	}
	value, err := password.Pwgen()
	if err != nil {
		return nil, err
	}
	password.Value = value
	return password, nil
}

// generatePassphrasePassword creates a passphrase with the selected word count.
func generatePassphrasePassword(words int) (*paw.PasswordSpec, error) {
	password := paw.NewPassphrasePassword()
	password.Length = words
	value, err := password.Pwgen()
	if err != nil {
		return nil, err
	}
	password.Value = value
	return password, nil
}

// generatePinPassword creates a numeric password with the selected length.
func generatePinPassword(length int) (*paw.PasswordSpec, error) {
	password := paw.NewPinPassword()
	password.Length = length
	password.Format = paw.DigitsFormat
	value, err := password.Pwgen()
	if err != nil {
		return nil, err
	}
	password.Value = value
	return password, nil
}

type pwGenRandomCmd struct {
	*PwGenCmd
	length  int
	symbols bool
}

type pwGenPassphraseCmd struct {
	*PwGenCmd
	words int
}

type pwGenPinCmd struct {
	*PwGenCmd
	length int
}

// newPwGenRandomCommand registers non-interactive random password generation.
func newPwGenRandomCommand(base *PwGenCmd) *urfave.Command {
	cmd := &pwGenRandomCmd{PwGenCmd: base}
	c := &urfave.Command{
		Name:      "random",
		Usage:     "Generates a random password",
		ArgsUsage: " ",
		Flags: []urfave.Flag{
			&urfave.IntFlag{Name: "length", Value: paw.NewRandomPassword().Length, Usage: "Password length"},
			&urfave.BoolFlag{Name: "no-symbols", Usage: "Exclude symbols"},
		},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the random password options.
func (cmd *pwGenRandomCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 0); err != nil {
		return ctx, err
	}
	cmd.length = c.Int("length")
	if cmd.length < 6 || cmd.length > 64 {
		return ctx, invalidArgumentsError("--length must be between 6 and 64")
	}
	cmd.symbols = !c.Bool("no-symbols")
	return ctx, nil
}

// Run generates and writes a random password.
func (cmd *pwGenRandomCmd) Run(ctx context.Context, _ *urfave.Command) error {
	password, err := generateRandomPassword(cmd.length, cmd.symbols)
	if err != nil {
		return err
	}
	return cmd.writePassword(ctx, password.Value)
}

// newPwGenPassphraseCommand registers non-interactive passphrase generation.
func newPwGenPassphraseCommand(base *PwGenCmd) *urfave.Command {
	cmd := &pwGenPassphraseCmd{PwGenCmd: base}
	c := &urfave.Command{
		Name:         "passphrase",
		Usage:        "Generates a passphrase",
		ArgsUsage:    " ",
		Flags:        []urfave.Flag{&urfave.IntFlag{Name: "words", Value: paw.NewPassphrasePassword().Length, Usage: "Number of words"}},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the passphrase word count.
func (cmd *pwGenPassphraseCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 0); err != nil {
		return ctx, err
	}
	cmd.words = c.Int("words")
	if cmd.words < 2 || cmd.words > 12 {
		return ctx, invalidArgumentsError("--words must be between 2 and 12")
	}
	return ctx, nil
}

// Run generates and writes a passphrase.
func (cmd *pwGenPassphraseCmd) Run(ctx context.Context, _ *urfave.Command) error {
	password, err := generatePassphrasePassword(cmd.words)
	if err != nil {
		return err
	}
	return cmd.writePassword(ctx, password.Value)
}

// newPwGenPinCommand registers non-interactive PIN generation.
func newPwGenPinCommand(base *PwGenCmd) *urfave.Command {
	cmd := &pwGenPinCmd{PwGenCmd: base}
	c := &urfave.Command{
		Name:         "pin",
		Usage:        "Generates a numeric PIN",
		ArgsUsage:    " ",
		Flags:        []urfave.Flag{&urfave.IntFlag{Name: "length", Value: paw.NewPinPassword().Length, Usage: "PIN length"}},
		OnUsageError: onUsageError,
	}
	c.Before = cmd.Validate
	c.Action = cmd.Run
	return c
}

// Validate checks the PIN length.
func (cmd *pwGenPinCmd) Validate(ctx context.Context, c *urfave.Command) (context.Context, error) {
	if err := validateArgs(c, 0, 0); err != nil {
		return ctx, err
	}
	cmd.length = c.Int("length")
	if cmd.length < 4 || cmd.length > 10 {
		return ctx, invalidArgumentsError("--length must be between 4 and 10")
	}
	return ctx, nil
}

// Run generates and writes a numeric PIN.
func (cmd *pwGenPinCmd) Run(ctx context.Context, _ *urfave.Command) error {
	password, err := generatePinPassword(cmd.length)
	if err != nil {
		return err
	}
	return cmd.writePassword(ctx, password.Value)
}
