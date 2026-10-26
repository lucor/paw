// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration

import (
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestMigrateSSHKeyConvertsRSAAndEd25519 checks key material and metadata survive conversion.
func TestMigrateSSHKeyConvertsRSAAndEd25519(t *testing.T) {
	keys := []struct {
		name string
		key  crypto.PrivateKey
	}{
		{name: "Ed25519", key: generateEd25519Key(t)},
		{name: "RSA", key: generateRSAKey(t)},
	}
	for _, test := range keys {
		t.Run(test.name, func(t *testing.T) {
			legacy := newLegacySSHKey(t, test.key, "")
			legacy.Name = "Deploy key"
			legacy.Metadata.Subtitle = "Saved subtitle"
			legacy.Comment = "deploy@example"
			legacy.AddToAgent = true
			legacy.Passphrase = &paw.Password{
				Value: "saved passphrase", Mode: paw.RandomPassword, Length: 24, Format: paw.DigitsFormat,
				Metadata: &paw.Metadata{Name: "ignored password metadata", Type: paw.PasswordItemType},
				Note:     &paw.Note{Value: "ignored password note"},
			}
			legacy.Note = &paw.Note{
				Value:    "SSH note",
				Metadata: &paw.Metadata{Name: "ignored note metadata", Type: paw.NoteItemType},
			}
			legacy.Metadata.Created = time.Unix(100, 123).UTC()
			legacy.Metadata.Modified = time.Unix(200, 456).UTC()

			item, err := MigrateSSHKey(legacy)
			require.NoError(t, err)
			require.NotEqual(t, legacy.ID(), item.ID)
			require.Equal(t, "Deploy key", item.Title)
			require.Equal(t, "Saved subtitle", *item.Subtitle)
			require.Equal(t, uint64(100), *item.CreationAt)
			require.Equal(t, uint64(200), *item.ModifiedAt)
			require.True(t, item.Metadata.AddToAgent)
			require.Equal(t, paw.SSHKeyItemType, item.Metadata.Type)
			require.Len(t, item.Credentials, 3)

			var credential cxf.SSHKey
			require.NoError(t, json.Unmarshal(item.Credentials[0], &credential))
			require.Equal(t, "deploy@example", credential.KeyComment)
			der, err := base64.RawURLEncoding.DecodeString(credential.PrivateKey)
			require.NoError(t, err)
			privateKey, err := x509.ParsePKCS8PrivateKey(der)
			require.NoError(t, err)
			signer, err := ssh.NewSignerFromKey(privateKey)
			require.NoError(t, err)
			message := []byte("Paw SSH migration check")
			signature, err := signer.Sign(rand.Reader, message)
			require.NoError(t, err)
			require.NoError(t, signer.PublicKey().Verify(message, signature))
			require.Equal(t, legacy.PublicKey, string(ssh.MarshalAuthorizedKey(signer.PublicKey())))
			require.Equal(t, legacy.Fingerprint, ssh.FingerprintSHA256(signer.PublicKey()))

			var passphrase cxf.BasicAuth
			require.NoError(t, json.Unmarshal(item.Credentials[1], &passphrase))
			require.Nil(t, passphrase.Username)
			require.Equal(t, "saved passphrase", passphrase.Password.Value)
			var extension struct {
				Name string `json:"name"`
				model.PasswordSettings
			}
			require.NoError(t, json.Unmarshal(passphrase.Password.Extensions[0], &extension))
			require.Equal(t, "paw.pm/password", extension.Name)
			require.Equal(t, model.PasswordSettings{Mode: paw.RandomPassword, Length: 24, Format: paw.DigitsFormat}, extension.PasswordSettings)
			var note cxf.Note
			require.NoError(t, json.Unmarshal(item.Credentials[2], &note))
			require.Equal(t, "SSH note", note.Content.Value)
			require.NotContains(t, string(item.Credentials[2]), "ignored")
			require.NoError(t, item.Validate())

			encoded, err := json.Marshal(item)
			require.NoError(t, err)
			var decoded model.Item
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, item.Metadata, decoded.Metadata)
			summary, err := decoded.Summary()
			require.NoError(t, err)
			require.Empty(t, summary.Credentials)
			require.Contains(t, string(summary.Extensions[0]), `"addToAgent":true`)
			summaryJSON, err := json.Marshal(summary)
			require.NoError(t, err)
			require.NotContains(t, string(summaryJSON), "saved passphrase")
			require.NotContains(t, string(summaryJSON), credential.PrivateKey)
			require.NotContains(t, string(summaryJSON), "paw.pm/password")
		})
	}
}

// TestMigrateSSHKeyHandlesPassphrases checks protected and unprotected key imports.
func TestMigrateSSHKeyHandlesPassphrases(t *testing.T) {
	key := generateEd25519Key(t)
	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "test", []byte("correct horse"))
	require.NoError(t, err)
	protected := pem.EncodeToMemory(block)
	for _, test := range []struct {
		name       string
		passphrase *paw.Password
		wantErr    bool
	}{
		{name: "missing", wantErr: true},
		{name: "wrong", passphrase: &paw.Password{Value: "wrong", Mode: paw.CustomPassword}, wantErr: true},
		{name: "correct", passphrase: &paw.Password{Value: "correct horse", Mode: paw.PassphrasePassword, Length: 4}, wantErr: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			legacy := newLegacySSHKey(t, key, "")
			legacy.PrivateKey = string(protected)
			legacy.Passphrase = test.passphrase
			item, err := MigrateSSHKey(legacy)
			if test.wantErr {
				require.Error(t, err)
				require.Nil(t, item)
				require.NotContains(t, err.Error(), "correct horse")
				return
			}
			require.NoError(t, err)
			require.NotNil(t, item)
		})
	}

	legacy := newLegacySSHKey(t, key, "")
	legacy.Passphrase = &paw.Password{Value: "saved but not used", Mode: paw.CustomPassword}
	item, err := MigrateSSHKey(legacy)
	require.NoError(t, err)
	var passphrase cxf.BasicAuth
	require.NoError(t, json.Unmarshal(item.Credentials[1], &passphrase))
	require.Equal(t, "saved but not used", passphrase.Password.Value)
}

// TestMigrateSSHKeyOptionalFields checks empty values and password modes are retained.
func TestMigrateSSHKeyOptionalFields(t *testing.T) {
	key := generateEd25519Key(t)
	for _, mode := range []paw.PasswordMode{paw.CustomPassword, paw.RandomPassword, paw.PassphrasePassword, paw.PinPassword} {
		t.Run(mode.String(), func(t *testing.T) {
			legacy := newLegacySSHKey(t, key, "")
			legacy.Passphrase = &paw.Password{Mode: mode, Length: 12, Format: paw.DigitsFormat}
			legacy.Note = &paw.Note{}
			item, err := MigrateSSHKey(legacy)
			require.NoError(t, err)
			require.Len(t, item.Credentials, 2)
			var passphrase cxf.BasicAuth
			require.NoError(t, json.Unmarshal(item.Credentials[1], &passphrase))
			require.Empty(t, passphrase.Password.Value)
			var settings model.PasswordSettings
			var extension struct {
				model.PasswordSettings
			}
			require.NoError(t, json.Unmarshal(passphrase.Password.Extensions[0], &extension))
			settings = extension.PasswordSettings
			require.Equal(t, model.PasswordSettings{Mode: mode, Length: 12, Format: paw.DigitsFormat}, settings)
		})
	}
	legacy := newLegacySSHKey(t, key, "")
	legacy.Passphrase = nil
	item, err := MigrateSSHKey(legacy)
	require.NoError(t, err)
	var passphrase cxf.BasicAuth
	require.NoError(t, json.Unmarshal(item.Credentials[1], &passphrase))
	require.Empty(t, passphrase.Password.Value)
	var extension struct {
		model.PasswordSettings
	}
	require.NoError(t, json.Unmarshal(passphrase.Password.Extensions[0], &extension))
	require.Equal(t, model.PasswordSettings{Mode: paw.CustomPassword}, extension.PasswordSettings)
}

// TestMigrateSSHKeyRejectsInvalidData checks migration errors do not expose secrets.
func TestMigrateSSHKeyRejectsInvalidData(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*paw.SSHKey)
	}{
		{name: "nil", setup: func(_ *paw.SSHKey) {}},
		{name: "missing metadata", setup: func(key *paw.SSHKey) { key.Metadata = nil }},
		{name: "invalid key", setup: func(key *paw.SSHKey) { key.PrivateKey = "private-secret" }},
		{name: "unsupported mode", setup: func(key *paw.SSHKey) {
			key.Passphrase.Mode = paw.PasswordMode(4)
			key.Passphrase.Value = "password-secret"
		}},
		{name: "invalid title", setup: func(key *paw.SSHKey) { key.Metadata.Name = "bad\xff" }},
		{name: "invalid subtitle", setup: func(key *paw.SSHKey) { key.Metadata.Subtitle = "bad\xff" }},
		{name: "public mismatch", setup: func(key *paw.SSHKey) {
			key.PublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA mismatch"
		}},
		{name: "fingerprint mismatch", setup: func(key *paw.SSHKey) { key.Fingerprint = "SHA256:mismatch" }},
		{name: "invalid note", setup: func(key *paw.SSHKey) { key.Note.Value = "bad\xff" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if test.name == "nil" {
				item, err := MigrateSSHKey(nil)
				require.Error(t, err)
				require.Nil(t, item)
				return
			}
			legacy := newLegacySSHKey(t, generateEd25519Key(t), "")
			legacy.Passphrase = &paw.Password{Mode: paw.CustomPassword}
			test.setup(legacy)
			item, err := MigrateSSHKey(legacy)
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "private-secret")
			require.NotContains(t, err.Error(), "password-secret")
		})
	}
}

// generateEd25519Key returns a test-only Ed25519 key.
func generateEd25519Key(t *testing.T) ed25519.PrivateKey {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	return key
}

// generateRSAKey returns a test-only RSA key.
func generateRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

// newLegacySSHKey builds a legacy item with a matching public key and fingerprint.
func newLegacySSHKey(t *testing.T, key crypto.PrivateKey, passphrase string) *paw.SSHKey {
	t.Helper()
	var block *pem.Block
	var err error
	if passphrase == "" {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			block = &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(rsaKey)}
		} else {
			block, err = ssh.MarshalPrivateKey(key, "legacy comment")
		}
	} else {
		block, err = ssh.MarshalPrivateKeyWithPassphrase(key, "legacy comment", []byte(passphrase))
	}
	require.NoError(t, err)
	publicKey, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	return &paw.SSHKey{
		Metadata:    &paw.Metadata{Type: paw.SSHKeyItemType, Name: "Legacy key", Created: time.Unix(10, 0), Modified: time.Unix(20, 0)},
		PrivateKey:  string(pem.EncodeToMemory(block)),
		PublicKey:   string(ssh.MarshalAuthorizedKey(publicKey.PublicKey())),
		Fingerprint: ssh.FingerprintSHA256(publicKey.PublicKey()),
		Passphrase:  &paw.Password{Mode: paw.CustomPassword},
		Note:        &paw.Note{},
	}
}
