// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestDecodeVault(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
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

func TestDecodeVaultPreservesMissingDates(t *testing.T) {
	vault, err := DecodeVault([]byte(`{"Name":"Personal"}`), nil)
	require.NoError(t, err)
	require.True(t, vault.Created.IsZero())
	require.True(t, vault.Modified.IsZero())
}

func TestDecodeItem(t *testing.T) {
	note := NewNote()
	note.Name, note.Value = "Note", "Synthetic note"
	password := NewPassword()
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

func TestDecodeRejectsInvalidJSON(t *testing.T) {
	for _, data := range []string{`{`, `{} {}`, `[]`, `null`} {
		t.Run(data, func(t *testing.T) {
			vault, err := DecodeVault([]byte(data), nil)
			require.Error(t, err)
			require.Nil(t, vault)
			item, err := DecodeItem([]byte(data), model.NoteItemType)
			require.Error(t, err)
			require.Nil(t, item)
		})
	}
}

func TestDecodeErrorsExcludeValues(t *testing.T) {
	_, err := DecodeVault([]byte(`{"Created":"synthetic private value"}`), nil)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic private value")
	_, err = DecodeItem([]byte(`{"metadata":{"created":"synthetic private value"}}`), model.NoteItemType)
	require.Error(t, err)
	require.NotContains(t, err.Error(), "synthetic private value")
}

func TestDecodeItemRejectsUnsupportedType(t *testing.T) {
	item, err := DecodeItem([]byte(`{}`), model.ItemType(999))
	require.ErrorContains(t, err, "unsupported legacy item type")
	require.Nil(t, item)
}
