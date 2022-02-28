package cli

import (
	"fmt"
	"os"

	"lucor.dev/paw/internal/paw"
)

// PwGenCmd generates a password
type PwGenCmd struct{}

// Name returns the one word command name
func (cmd *PwGenCmd) Name() string {
	return "pwgen"
}

// Description returns the command description
func (cmd *PwGenCmd) Description() string {
	return "Generates a password"
}

// Usage displays the command usage
func (cmd *PwGenCmd) Usage() {
	template := `Usage: paw-cli pwgen [OPTION]

{{ . }}

Options:
  -h, --help  Displays this help and exit
`
	printUsage(template, cmd.Description())
}

// Parse parses the arguments and set the usage for the command
func (cmd *PwGenCmd) Parse(args []string) error {
	flags, err := newCommonFlags()
	if err != nil {
		return err
	}

	flagSet.Parse(args)
	if flags.Help {
		cmd.Usage()
		os.Exit(0)
	}

	return nil
}

// Run runs the command
func (cmd *PwGenCmd) Run(s paw.Storage) error {
	modes := []paw.PasswordMode{
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	}
	password, err := cmd.Pwgen(nil, modes, paw.RandomPassword)
	if err != nil {
		return err
	}

	fmt.Println(password.Value)
	return nil
}

func (cmd *PwGenCmd) Pwgen(key *paw.Key, modes []paw.PasswordMode, defaultMode paw.PasswordMode) (*paw.Password, error) {

	var err error

	if key == nil {
		key, err = paw.MakeOneTimeKey()
		if err != nil {
			return nil, err
		}
	}

	choice, err := askPasswordMode("Password type", modes, defaultMode)
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
	length, err := askIntWithDefaultAndRange("Password length", p.Length, 6, 64)
	if err != nil {
		return nil, err
	}
	p.Length = length
	p.Format = paw.UppercaseFormat | paw.LowercaseFormat | paw.DigitsFormat

	wantSymbols, err := askYesNo("Password should contains symbols?", true)
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
	v, err := askPasswordWithConfirm()
	if err != nil {
		return nil, err
	}
	p.Value = v
	return p, nil
}

func (cmd *PwGenCmd) makePassphrasePassword(key *paw.Key) (*paw.Password, error) {
	p := paw.NewPassphrasePassword()
	length, err := askIntWithDefaultAndRange("Passphrase words", p.Length, 2, 12)
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
	length, err := askIntWithDefaultAndRange("Pin length", p.Length, 4, 10)
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
