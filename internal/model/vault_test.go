package model

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
)

const testVaultJSON = `{"version":{"major":1,"minor":0},"exporterRpId":"paw.pm","exporterDisplayName":"Paw","timestamp":1705228800,"accounts":[{"id":"AAAAAAAAAAAAAAAAAAAAAA","username":"","email":"","collections":[],"items":[{"id":"AQEBAQEBAQEBAQEBAQEBAQ","title":"A note","credentials":[]}],"extensions":[{"name":"paw.pm/vault","schemaVersion":1,"vaultName":"Personal","creationAt":0},{"name":"example.com/future","schemaVersion":"opaque","counter":9007199254740993}]}]}`

// TestNewVault checks new vaults use one valid CXF account and Paw extension.
func TestNewVault(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	before := uint64(time.Now().Unix())
	vault, err := NewVault(key, "Personal")
	require.NoError(t, err)
	require.Same(t, key, vault.Key())
	require.NoError(t, vault.Validate())
	require.Equal(t, cxf.Version{Major: 1, Minor: 0}, vault.Version)
	require.Equal(t, "paw.pm", vault.ExporterRPID)
	require.Equal(t, "Paw", vault.ExporterDisplayName)
	require.GreaterOrEqual(t, vault.Timestamp, before)
	require.Len(t, vault.Accounts, 1)
	account := vault.Accounts[0]
	require.Equal(t, "", account.Username)
	require.Equal(t, "", account.Email)
	require.Empty(t, account.Collections)
	require.Empty(t, account.Items)
	id, err := base64.RawURLEncoding.Strict().DecodeString(account.ID)
	require.NoError(t, err)
	require.Len(t, id, 16)
	name, err := vault.Name()
	require.NoError(t, err)
	require.Equal(t, "Personal", name)
	require.Equal(t, "Personal", vault.Metadata.VaultName)
	creationAt, err := vault.CreationAt()
	require.NoError(t, err)
	require.NotNil(t, creationAt)
	require.Equal(t, vault.Timestamp, *creationAt)

	encoded, err := json.Marshal(vault)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), key.String())
	decoded, err := cxf.Decode(bytes.NewReader(encoded))
	require.NoError(t, err)
	require.Equal(t, vault.Header, *decoded)
}

// TestVaultMetadataIsCachedAndMarshaled checks metadata stays in memory and edits reach the extension.
func TestVaultMetadataIsCachedAndMarshaled(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	data := strings.Replace(testVaultJSON, `"creationAt":0`, `"creationAt":0,"future":{"keep":true}`, 1)
	vault, err := DecodeVault([]byte(data), key)
	require.NoError(t, err)
	require.Equal(t, "Personal", vault.Metadata.VaultName)
	require.NotNil(t, vault.Metadata.CreationAt)
	require.Zero(t, *vault.Metadata.CreationAt)

	vault.Metadata.VaultName = "Work"
	creationAt := uint64(42)
	vault.Metadata.CreationAt = &creationAt
	encoded, err := json.Marshal(vault)
	require.NoError(t, err)

	var document cxf.Header
	require.NoError(t, json.Unmarshal(encoded, &document))
	require.JSONEq(t, `{"name":"paw.pm/vault","schemaVersion":1,"vaultName":"Work","creationAt":42,"future":{"keep":true}}`, string(document.Accounts[0].Extensions[0]))
	require.JSONEq(t, `{"name":"example.com/future","schemaVersion":"opaque","counter":9007199254740993}`, string(document.Accounts[0].Extensions[1]))
}

// TestDecodeVaultPreservesUnknownExtensions checks raw account extensions survive.
func TestDecodeVaultPreservesUnknownExtensions(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := DecodeVault([]byte(testVaultJSON), key)
	require.NoError(t, err)
	require.Same(t, key, vault.Key())
	name, err := vault.Name()
	require.NoError(t, err)
	require.Equal(t, "Personal", name)
	creationAt, err := vault.CreationAt()
	require.NoError(t, err)
	require.NotNil(t, creationAt)
	require.Zero(t, *creationAt)

	encoded, err := json.Marshal(vault)
	require.NoError(t, err)
	var decoded map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	var accounts []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(decoded["accounts"], &accounts))
	var extensions []json.RawMessage
	require.NoError(t, json.Unmarshal(accounts[0]["extensions"], &extensions))
	require.Len(t, extensions, 2)
	require.JSONEq(t, `{"name":"paw.pm/vault","schemaVersion":1,"vaultName":"Personal","creationAt":0}`, string(extensions[0]))
	require.JSONEq(t, `{"name":"example.com/future","schemaVersion":"opaque","counter":9007199254740993}`, string(extensions[1]))

	withoutCreationAt := strings.Replace(testVaultJSON, `,"creationAt":0`, "", 1)
	vault, err = DecodeVault([]byte(withoutCreationAt), key)
	require.NoError(t, err)
	creationAt, err = vault.CreationAt()
	require.NoError(t, err)
	require.Nil(t, creationAt)
}

// TestDecodeVaultRejectsInvalidData checks CXF and Paw vault rules.
func TestDecodeVaultRejectsInvalidData(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	for _, test := range []struct {
		name string
		from string
		to   string
		want string
	}{
		{name: "missing Paw extension", from: `"paw.pm/vault"`, to: `"example.com/other"`, want: "missing Paw vault metadata"},
		{name: "duplicate Paw extension", from: `"name":"example.com/future"`, to: `"name":"paw.pm/vault"`, want: "duplicate Paw vault metadata"},
		{name: "missing schema", from: `"schemaVersion":1,`, to: ``, want: "unsupported Paw storage schema version"},
		{name: "unsupported schema", from: `"schemaVersion":1`, to: `"schemaVersion":2`, want: "unsupported Paw storage schema version"},
		{name: "missing vault name", from: `"vaultName":"Personal",`, to: ``, want: "invalid Paw vault metadata"},
		{name: "unsupported CXF minor", from: `"minor":0`, to: `"minor":1`, want: "unsupported CXF version 1.1"},
		{name: "unsupported CXF major", from: `"major":1`, to: `"major":2`, want: "unsupported major version 2"},
		{name: "credentials in summary", from: `"credentials":[]`, to: `"credentials":[{"type":"future-credential"}]`, want: "vault contains item credentials"},
		{name: "missing account email", from: `"email":"",`, to: ``, want: "account.email: required field is missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := strings.Replace(testVaultJSON, test.from, test.to, 1)
			require.NotEqual(t, testVaultJSON, data)
			_, err := DecodeVault([]byte(data), key)
			require.ErrorContains(t, err, test.want)
			require.NotContains(t, err.Error(), "synthetic-secret")
		})
	}

	vault, err := DecodeVault([]byte(testVaultJSON), key)
	require.NoError(t, err)
	secondAccount := vault.Accounts[0]
	secondAccount.ID = "AQ"
	vault.Accounts = append(vault.Accounts, secondAccount)
	require.ErrorContains(t, vault.Validate(), "exactly one account")

	_, err = DecodeVault([]byte(testVaultJSON), nil)
	require.ErrorContains(t, err, "key is nil")
}

// TestVaultUnmarshalIsAtomic checks failed decoding leaves the receiver unchanged.
func TestVaultUnmarshalIsAtomic(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := NewVault(key, "Original")
	require.NoError(t, err)
	original := vault.Header

	err = json.Unmarshal([]byte(`{"version":{"major":1,"minor":0}}`), vault)

	require.Error(t, err)
	require.Equal(t, original, vault.Header)
	require.Same(t, key, vault.Key())

	require.NoError(t, json.Unmarshal([]byte(testVaultJSON), vault))
	require.Nil(t, vault.Key())
}

// TestNewVaultRejectsInvalidInput checks a missing key and invalid text fail.
func TestNewVaultRejectsInvalidInput(t *testing.T) {
	_, err := NewVault(nil, "Personal")
	require.ErrorContains(t, err, "key is nil")

	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := NewVault(key, "private\xff")
	require.Error(t, err)
	require.Nil(t, vault)
	require.NotContains(t, err.Error(), "private")
}
