// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"strings"

	"go.lucor.dev/paw/internal/age"
)

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
)

func (pm PasswordMode) String() string {
	switch pm {
	case CustomPassword:
		return "Custom"
	case RandomPassword:
		return "Random"
	case PinPassword:
		return "Pin"
	case PassphrasePassword:
		return "Passphrase"
	}
	return fmt.Sprintf("Unknown password mode (%d)", pm)
}

// PasswordSpec holds temporary generator settings and its generated value.
type PasswordSpec struct {
	Value  string       `json:"value,omitempty"`
	Format Format       `json:"format,omitempty"`
	Length int          `json:"length,omitempty"`
	Mode   PasswordMode `json:"mode,omitempty"`
}

func NewRandomPassword() *PasswordSpec {
	password := &PasswordSpec{}
	password.Mode = RandomPassword
	password.Format = RandomPasswordDefaultFormat
	password.Length = RandomPasswordDefaultLength
	return password
}

func NewPinPassword() *PasswordSpec {
	password := &PasswordSpec{}
	password.Mode = PinPassword
	password.Format = PinPasswordDefaultFormat
	password.Length = PinPasswordDefaultLength
	return password
}

func NewPassphrasePassword() *PasswordSpec {
	password := &PasswordSpec{}
	password.Mode = PassphrasePassword
	password.Length = PassphrasePasswordDefaultLength
	return password
}

func NewCustomPassword() *PasswordSpec {
	password := &PasswordSpec{}
	password.Mode = CustomPassword
	return password
}

func (p *PasswordSpec) Template() (string, error) {
	ruler, err := NewRule(p.Length, p.Format)
	if err != nil {
		return "", err
	}
	return ruler.Template()
}

// Pwgen generates a password using a supported mode.
func (p *PasswordSpec) Pwgen() (string, error) {
	switch p.Mode {
	case CustomPassword, RandomPassword, PassphrasePassword, PinPassword:
	default:
		return "", fmt.Errorf("unsupported password mode %d", p.Mode)
	}
	if p.Mode == CustomPassword {
		return p.Value, nil
	}
	if p.Mode == PassphrasePassword {
		var words []string
		for i := 0; i < p.Length; i++ {
			words = append(words, age.RandomWord())
		}
		return strings.Join(words, "-"), nil
	}
	return p.randomSecret(rand.Reader)
}

func (p *PasswordSpec) randomSecret(reader io.Reader) (string, error) {
	if p.Length <= 0 {
		return "", fmt.Errorf("password length must be greater than zero")
	}
	chars, err := p.Template()
	if err != nil {
		return "", fmt.Errorf("could not create password character set: %w", err)
	}
	if len(chars) == 0 {
		return "", fmt.Errorf("password character set cannot be empty")
	}
	alphabet := []byte(chars)
	secret := make([]byte, 0, p.Length)
	var candidate [1]byte
	for len(secret) < p.Length {
		if _, err := io.ReadFull(reader, candidate[:]); err != nil {
			return "", fmt.Errorf("could not read random password data: %w", err)
		}
		if bytes.Contains(alphabet, candidate[:]) {
			secret = append(secret, candidate[0])
		}
	}
	return string(secret), nil
}
