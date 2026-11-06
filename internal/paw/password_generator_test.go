// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestPasswordRejectsRemovedMode checks old stateless values cannot generate passwords.
func TestPasswordRejectsRemovedMode(t *testing.T) {
	for _, data := range []string{
		`{"mode":4,"length":16,"format":15}`,
		`{"mode":99,"length":16,"format":15}`,
	} {
		var password PasswordSpec
		require.NoError(t, json.Unmarshal([]byte(data), &password))
		value, err := password.Pwgen()
		require.ErrorContains(t, err, "unsupported password mode")
		require.Empty(t, value)
	}
	require.Equal(t, "Unknown password mode (4)", PasswordMode(4).String())
}

// TestPasswordGeneration checks supported generators still produce usable passwords.
func TestPasswordGeneration(t *testing.T) {
	for _, password := range []*PasswordSpec{NewRandomPassword(), NewPinPassword(), NewPassphrasePassword()} {
		t.Run(password.Mode.String(), func(t *testing.T) {
			value, err := password.Pwgen()
			require.NoError(t, err)
			if password.Mode == PassphrasePassword {
				require.Len(t, strings.Split(value, "-"), password.Length)
				return
			}
			require.Len(t, value, password.Length)
			template, err := password.Template()
			require.NoError(t, err)
			for _, char := range value {
				require.Contains(t, template, string(char))
			}
		})
	}
}

func TestRandomSecretUsesRejectionSamplingAndPreservesPINLeadingZero(t *testing.T) {
	pin := &PasswordSpec{Mode: PinPassword, Format: DigitsFormat, Length: 3}
	value, err := pin.randomSecret(bytes.NewReader([]byte{'x', '0', '1', '2'}))
	require.NoError(t, err)
	require.Equal(t, "012", value)
}

func TestRandomSecretRejectsEmptyAlphabetBeforeReading(t *testing.T) {
	reader := &failingRandomReader{}
	password := &PasswordSpec{Mode: RandomPassword, Length: 8}
	value, err := password.randomSecret(reader)
	require.ErrorContains(t, err, "character set cannot be empty")
	require.Empty(t, value)
	require.Zero(t, reader.reads)
}

func TestRandomSecretReadFailureDoesNotReturnPartialPassword(t *testing.T) {
	password := &PasswordSpec{Mode: RandomPassword, Format: LowercaseFormat, Length: 2}
	reader := io.MultiReader(bytes.NewReader([]byte{'a'}), &failingRandomReader{})
	value, err := password.randomSecret(reader)
	require.ErrorContains(t, err, "random password data")
	require.Empty(t, value)
}

type failingRandomReader struct {
	reads int
}

func (r *failingRandomReader) Read([]byte) (int, error) {
	r.reads++
	return 0, errors.New("random source unavailable")
}
