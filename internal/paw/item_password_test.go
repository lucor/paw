// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"encoding/json"
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
		var password Password
		require.NoError(t, json.Unmarshal([]byte(data), &password))
		value, err := password.Pwgen(nil)
		require.ErrorContains(t, err, "unsupported password mode")
		require.Empty(t, value)
	}
	require.Equal(t, "Unknown password mode (4)", PasswordMode(4).String())
}

// TestPasswordGeneration checks supported generators still produce usable passwords.
func TestPasswordGeneration(t *testing.T) {
	key, err := MakeOneTimeKey()
	require.NoError(t, err)
	for _, password := range []*Password{NewRandomPassword(), NewPinPassword(), NewPassphrasePassword()} {
		t.Run(password.Mode.String(), func(t *testing.T) {
			require.Nil(t, password.Salt())
			value, err := password.Pwgen(key)
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
