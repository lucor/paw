// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

// TestNewSSHKeyBuildsCredentials checks SSH and passphrase credentials.
func TestNewSSHKeyBuildsCredentials(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	for _, mode := range []paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword} {
		t.Run(mode.String(), func(t *testing.T) {
			settings := PasswordSettings{Mode: mode, Length: 16, Format: paw.RandomPasswordDefaultFormat}
			item, err := NewSSHKey("Deploy key", cxf.SSHKey{
				Type: cxf.CredentialTypeSSHKey, KeyType: "ssh-ed25519",
				PrivateKey: base64.RawURLEncoding.EncodeToString(privateDER), KeyComment: "deploy@example",
			}, "secret passphrase", settings)
			require.NoError(t, err)
			require.Equal(t, paw.SSHKeyItemType, item.Metadata.Type)
			require.Len(t, item.Credentials, 2)

			var key cxf.SSHKey
			require.NoError(t, json.Unmarshal(item.Credentials[0], &key))
			require.Equal(t, "ssh-ed25519", key.KeyType)
			require.Equal(t, "deploy@example", key.KeyComment)
			var passphrase cxf.BasicAuth
			require.NoError(t, json.Unmarshal(item.Credentials[1], &passphrase))
			require.Nil(t, passphrase.Username)
			require.Equal(t, "secret passphrase", passphrase.Password.Value)
			var extension passwordExtension
			require.NoError(t, json.Unmarshal(passphrase.Password.Extensions[0], &extension))
			require.Equal(t, settings, extension.PasswordSettings)
		})
	}
}

// TestNewSSHKeyRejectsInvalidData checks invalid keys and modes fail safely.
func TestNewSSHKeyRejectsInvalidData(t *testing.T) {
	for _, test := range []struct {
		name string
		key  cxf.SSHKey
		mode paw.PasswordMode
	}{
		{name: "invalid key", key: cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: "ssh-ed25519", PrivateKey: "bad"}},
		{name: "unsupported mode", key: validTestSSHKey(t), mode: paw.PasswordMode(4)},
	} {
		t.Run(test.name, func(t *testing.T) {
			item, err := NewSSHKey("Key", test.key, "do-not-report", PasswordSettings{Mode: test.mode})
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "do-not-report")
		})
	}
}

// validTestSSHKey returns a synthetic CXF SSH credential.
func validTestSSHKey(t *testing.T) cxf.SSHKey {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	require.NoError(t, err)
	return cxf.SSHKey{Type: cxf.CredentialTypeSSHKey, KeyType: "ssh-ed25519", PrivateKey: base64.RawURLEncoding.EncodeToString(der)}
}
