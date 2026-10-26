// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestDecodeVault checks saved metadata and the runtime key are preserved.
func TestDecodeVault(t *testing.T) {
	key, err := MakeOneTimeKey()
	require.NoError(t, err)
	vault := NewVault(key, "Personal")
	note := NewNote()
	note.Name, note.Value = "Example", "Synthetic note"
	require.NoError(t, vault.AddItem(note))
	data, err := json.Marshal(vault)
	require.NoError(t, err)

	decoded, err := DecodeVault(data, key)
	require.NoError(t, err)
	require.Equal(t, vault, decoded)
	require.Same(t, key, decoded.Key())
}

// TestDecodeVaultPreservesMissingDates checks reading does not invent saved dates.
func TestDecodeVaultPreservesMissingDates(t *testing.T) {
	vault, err := DecodeVault([]byte(`{"Name":"Personal"}`), nil)
	require.NoError(t, err)
	require.True(t, vault.Created.IsZero())
	require.True(t, vault.Modified.IsZero())
}

// TestDecodeItem checks all supported legacy types retain their saved fields.
func TestDecodeItem(t *testing.T) {
	note := NewNote()
	note.Name, note.Value = "Note", "Synthetic note"
	password := NewRandomPassword()
	password.Name, password.Value = "Password", "Synthetic password"
	login := NewLogin()
	login.Name, login.Username = "Login", "example user"
	login.Password.Value = "Synthetic login password"
	login.TOTP.Secret = "JBSWY3DPEHPK3PXP"
	sshKey := NewSSHKey()
	sshKey.Name, sshKey.PrivateKey = "SSH key", "Synthetic private key"
	sshKey.AddToAgent = true
	for _, item := range []Item{note, password, login, sshKey} {
		t.Run(item.GetMetadata().Type.String(), func(t *testing.T) {
			data, err := json.Marshal(item)
			require.NoError(t, err)
			decoded, err := DecodeItem(data, item.GetMetadata().Type)
			require.NoError(t, err)
			require.Equal(t, item, decoded)
		})
	}
}

// TestDecodeLegacyRejectsInvalidJSON checks incomplete or extra documents fail.
func TestDecodeLegacyRejectsInvalidJSON(t *testing.T) {
	for _, data := range []string{`{`, `{} {}`, `[]`, `null`} {
		t.Run(data, func(t *testing.T) {
			vault, err := DecodeVault([]byte(data), nil)
			require.Error(t, err)
			require.Nil(t, vault)
			item, err := DecodeItem([]byte(data), NoteItemType)
			require.Error(t, err)
			require.Nil(t, item)
		})
	}
}

// TestDecodeLegacyErrorsExcludeValues checks date decoder errors cannot leak secrets.
func TestDecodeLegacyErrorsExcludeValues(t *testing.T) {
	_, err := DecodeVault([]byte(`{"Created":"synthetic private value"}`), nil)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic private value")
	_, err = DecodeItem([]byte(`{"metadata":{"created":"synthetic private value"}}`), NoteItemType)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic private value")
}

// TestDecodeItemRejectsUnsupportedType checks unknown records cannot be skipped silently.
func TestDecodeItemRejectsUnsupportedType(t *testing.T) {
	item, err := DecodeItem([]byte(`{}`), ItemType(999))
	require.ErrorContains(t, err, "unsupported legacy item type")
	require.Nil(t, item)
}
