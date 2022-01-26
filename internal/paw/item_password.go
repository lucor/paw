package paw

import (
	"fmt"
	"strings"
	"time"

	"lucor.dev/paw/internal/age"
)

// Declare conformity to Item interface
var _ Item = (*Password)(nil)

// Declare conformity to Seeder interface
var _ Seeder = (*Password)(nil)

const (
	RandomPasswordDefaultLength     = 16
	RandomPasswordMinLength         = 8
	RandomPasswordMaxLength         = 120
	RandomPasswordDefaultFormat     = LowercaseFormat | DigitsFormat | SymbolsFormat | UppercaseFormat
	PinPasswordDefaultLength        = 4
	PinPasswordMinLength            = 3
	PinPasswordMaxLength            = 10
	PinPasswordDefaultFormat        = DigitsFormat
	PassphrasePasswordDefaultLength = 4
	PassphrasePasswordMinLength     = 3
	PassphrasePasswordMaxLength     = 12
)

type PasswordMode uint32

const (
	CustomPassword     PasswordMode = 0
	RandomPassword     PasswordMode = 1
	PassphrasePassword PasswordMode = 2
	PinPassword        PasswordMode = 3
	StatelessPassword  PasswordMode = 4
)

func (pm PasswordMode) String() string {
	switch pm {
	case CustomPassword:
		return "Custom"
	case RandomPassword:
		return "Random"
	case StatelessPassword:
		return "Stateless"
	case PinPassword:
		return "Pin"
	case PassphrasePassword:
		return "Passphrase"
	}
	return fmt.Sprintf("Unknown password mode (%d)", pm)
}

type Password struct {
	Value  string       `json:"value,omitempty"`
	Format Format       `json:"format,omitempty"`
	Length int          `json:"length,omitempty"`
	Mode   PasswordMode `json:"mode,omitempty"`

	*Metadata `json:"metadata,omitempty"`
	*Note     `json:"note,omitempty"`
}

func NewPassword() *Password {
	now := time.Now()
	return &Password{
		Metadata: &Metadata{
			Type:     PasswordItemType,
			Created:  now,
			Modified: now,
		},
		Note: &Note{},
	}
}

func NewRandomPassword() *Password {
	password := NewPassword()
	password.Mode = RandomPassword
	password.Format = RandomPasswordDefaultFormat
	password.Length = RandomPasswordDefaultLength
	return password
}

func NewPinPassword() *Password {
	password := NewPassword()
	password.Mode = PinPassword
	password.Format = PinPasswordDefaultFormat
	password.Length = PinPasswordDefaultLength
	return password
}

func NewPassphrasePassword() *Password {
	password := NewPassword()
	password.Mode = PassphrasePassword
	password.Length = PassphrasePasswordDefaultLength
	return password
}

func NewCustomPassword() *Password {
	password := NewPassword()
	password.Mode = CustomPassword
	return password
}

// Implemets Seeder interface

func (p *Password) Salt() []byte {
	if p.Mode == StatelessPassword {
		return []byte(p.ID())
	}
	return nil
}

func (p *Password) Info() []byte {
	return nil
}

func (p *Password) Template() (string, error) {
	ruler, err := NewRule(p.Length, p.Format)
	if err != nil {
		return "", err
	}
	return ruler.Template()
}

func (p *Password) Len() int {
	return p.Length
}

func (p *Password) Pwgen(key *Key) (string, error) {
	if p.Mode == PassphrasePassword {
		var words []string
		for i := 0; i < p.Length; i++ {
			words = append(words, age.RandomWord())
		}
		return strings.Join(words, "-"), nil
	}
	secret, err := key.Secret(p)
	if err != nil {
		return "", fmt.Errorf("could not generate password: %w", err)
	}
	return secret, nil
}
