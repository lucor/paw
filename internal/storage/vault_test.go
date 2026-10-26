package storage

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// TestDecryptVault checks encrypted files decode as complete CXF vaults.
func TestDecryptVault(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)

	var encrypted bytes.Buffer
	require.NoError(t, encrypt(key, &encrypted, vault))
	require.NotContains(t, encrypted.String(), "Personal")

	decoded, err := decryptVault(key, bytes.NewReader(encrypted.Bytes()))
	require.NoError(t, err)
	require.Equal(t, vault.Header, decoded.Header)
	require.Same(t, key, decoded.Key())
}

// TestDecryptVaultRejectsTrailingJSON checks the reader accepts only one document.
func TestDecryptVaultRejectsTrailingJSON(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	data, err := json.Marshal(vault)
	require.NoError(t, err)
	data = append(data, []byte(` {}`)...)

	var encrypted bytes.Buffer
	writeEncrypted(t, key, &encrypted, data)

	_, err = decryptVault(key, bytes.NewReader(encrypted.Bytes()))
	require.ErrorContains(t, err, "could not decode vault")
}

// TestDecryptVaultRejectsTruncatedAge checks age authentication errors are returned.
func TestDecryptVaultRejectsTruncatedAge(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	var encrypted bytes.Buffer
	require.NoError(t, encrypt(key, &encrypted, vault))
	truncated := encrypted.Bytes()[:encrypted.Len()-1]

	_, err = decryptVault(key, bytes.NewReader(truncated))
	require.Error(t, err)
}

// TestDecryptItem checks encrypted CXF items round-trip and match their file ID.
func TestDecryptItem(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	item, err := model.NewNote("Title", "Content")
	require.NoError(t, err)
	var encrypted bytes.Buffer
	require.NoError(t, encrypt(key, &encrypted, item))
	require.NotContains(t, encrypted.String(), "Content")

	decoded, err := decryptItem(key, bytes.NewReader(encrypted.Bytes()), item.ID)
	require.NoError(t, err)
	require.Equal(t, item.ID, decoded.ID)
	require.Equal(t, item.Title, decoded.Title)
	require.Equal(t, item.Credentials, decoded.Credentials)
	require.Equal(t, item.Metadata, decoded.Metadata)
}

// TestDecryptItemRejectsWrongID checks the item ID matches the filename ID.
func TestDecryptItemRejectsWrongID(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	item, err := model.NewNote("Title", "Content")
	require.NoError(t, err)
	var encrypted bytes.Buffer
	require.NoError(t, encrypt(key, &encrypted, item))

	_, err = decryptItem(key, bytes.NewReader(encrypted.Bytes()), "another-id")
	require.ErrorContains(t, err, "does not match expected ID")
}

// TestDecryptItemRejectsTrailingJSON checks the item reader rejects extra documents.
func TestDecryptItemRejectsTrailingJSON(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	item, err := model.NewNote("Title", "Content")
	require.NoError(t, err)
	data, err := json.Marshal(item)
	require.NoError(t, err)
	data = append(data, []byte(` {}`)...)
	var encrypted bytes.Buffer
	writeEncrypted(t, key, &encrypted, data)

	_, err = decryptItem(key, bytes.NewReader(encrypted.Bytes()), item.ID)
	require.ErrorContains(t, err, "could not decode item")
}

// writeEncrypted encrypts raw JSON for reader failure tests.
func writeEncrypted(t *testing.T, key *paw.Key, dst *bytes.Buffer, data []byte) {
	t.Helper()
	writer, err := key.Encrypt(dst)
	require.NoError(t, err)
	_, writeErr := writer.Write(data)
	closeErr := writer.Close()
	require.NoError(t, writeErr)
	require.NoError(t, closeErr)
}
