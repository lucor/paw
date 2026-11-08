// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
)

func TestFileCredentialsRoundTripAndPreserveOtherCredentials(t *testing.T) {
	item, err := NewNote("Attachments", "note")
	require.NoError(t, err)
	original := append(jsontext.Value(nil), item.Credentials[0]...)
	item.Credentials = append(item.Credentials, jsontext.Value(`{"type":"future","value":42}`))

	first := testFileCredential(t, "same.txt", "first")
	second := testFileCredential(t, "same.txt", "second")
	require.NoError(t, item.AddFile(first))
	require.NoError(t, item.AddFile(second))
	files, err := item.Files()
	require.NoError(t, err)
	require.Equal(t, []cxf.File{first, second}, files)
	empty := testFileCredential(t, "empty.dat", "")
	empty.DecryptedSize = 0
	require.NoError(t, item.AddFile(empty))
	files, err = item.Files()
	require.NoError(t, err)
	require.Equal(t, []cxf.File{first, second, empty}, files)

	files[0].Name = "changed.txt"
	files, err = item.Files()
	require.NoError(t, err)
	require.Len(t, files, 3)
	require.Equal(t, "same.txt", files[0].Name)
	require.Equal(t, original, item.Credentials[0])
	require.Equal(t, jsontext.Value(`{"type":"future","value":42}`), item.Credentials[1])

	data, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	var decoded Item
	require.NoError(t, json.Unmarshal(data, &decoded))
	got, err := decoded.Files()
	require.NoError(t, err)
	require.Equal(t, []cxf.File{first, second, empty}, got)
	require.NoError(t, decoded.RemoveFile(first.ID))
	got, err = decoded.Files()
	require.NoError(t, err)
	require.Equal(t, []cxf.File{second, empty}, got)
	require.Equal(t, original, decoded.Credentials[0])
	require.Equal(t, jsontext.Value(`{"type":"future","value":42}`), decoded.Credentials[1])
	require.Equal(t, jsontext.Value(encodedCredential(t, second)), decoded.Credentials[2])
	require.Equal(t, jsontext.Value(encodedCredential(t, empty)), decoded.Credentials[3])
}

func TestFileCredentialValidationAndFailedOperationsAreAtomic(t *testing.T) {
	item, err := NewNote("Attachments", "note")
	require.NoError(t, err)
	valid := testFileCredential(t, "safe.txt", "one")
	require.NoError(t, item.AddFile(valid))

	before, err := json.Marshal(item.Credentials, json.Deterministic(true))
	require.NoError(t, err)
	require.ErrorContains(t, item.AddFile(valid), "already exists")
	require.ErrorContains(t, item.RemoveFile("missing"), "does not exist")
	invalid := valid
	invalid.IntegrityHash = "invalid"
	require.Error(t, item.AddFile(invalid))
	invalid = valid
	invalid.ID = "invalid"
	require.Error(t, item.AddFile(invalid))
	after, err := json.Marshal(item.Credentials, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, before, after)

	duplicate := append([]jsontext.Value(nil), item.Credentials...)
	encoded, err := json.Marshal(valid, json.Deterministic(true))
	require.NoError(t, err)
	duplicate = append(duplicate, encoded)
	item.Credentials = duplicate
	_, err = item.Files()
	require.ErrorContains(t, err, "duplicate")
	require.Error(t, item.RemoveFile(valid.ID))

	item.Credentials = []jsontext.Value{jsontext.Value(`{"type":"file","id":"bad"}`)}
	_, err = item.Files()
	require.Error(t, err)
	before, err = json.Marshal(item.Credentials, json.Deterministic(true))
	require.NoError(t, err)
	require.Error(t, item.RemoveFile("some-id"))
	after, err = json.Marshal(item.Credentials, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestFileCredentialNativeCategoryAndHistoryBehavior(t *testing.T) {
	file := testFileCredential(t, "history.bin", "history-file")
	for _, build := range []struct {
		name string
		item func() (*Item, error)
	}{
		{name: "note", item: func() (*Item, error) { return NewNote("Note", "text") }},
		{name: "login", item: func() (*Item, error) {
			return NewLogin("Login", "user", "pass", PasswordSettings{Mode: paw.CustomPassword})
		}},
	} {
		t.Run(build.name, func(t *testing.T) {
			item, err := build.item()
			require.NoError(t, err)
			before, err := item.ContentHash()
			require.NoError(t, err)
			require.NoError(t, item.AddFile(file))
			after, err := item.ContentHash()
			require.NoError(t, err)
			require.NotEqual(t, before, after)
			previous, err := build.item()
			require.NoError(t, err)
			previous.ID = item.ID
			previous.CreationAt = item.CreationAt
			previous.ModifiedAt = item.ModifiedAt
			require.NoError(t, previous.AddFile(file))
			require.NoError(t, item.RemoveFile(file.ID))
			updated, changed, err := PrepareItemEdit(previous, item, 100, 100)
			require.NoError(t, err)
			require.True(t, changed)
			files, err := updated.Files()
			require.NoError(t, err)
			require.Empty(t, files)
			history, err := updated.History()
			require.NoError(t, err)
			require.Len(t, history.Entries, 1)
			restored, changed, err := PrepareItemRestore(updated, 0, &history.Entries[0], 200)
			require.NoError(t, err)
			require.True(t, changed)
			files, err = restored.Files()
			require.NoError(t, err)
			require.Equal(t, []cxf.File{file}, files)
			summary, err := restored.Summary()
			require.NoError(t, err)
			require.Empty(t, summary.Credentials)
		})
	}

	password, err := NewPassword("Password", "secret", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	ssh, err := NewSSHKey("SSH", validTestSSHKey(t), "passphrase", PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	for _, item := range []*Item{password, ssh} {
		before, err := json.Marshal(item, json.Deterministic(true))
		require.NoError(t, err)
		require.Error(t, item.AddFile(file))
		require.Error(t, item.RemoveFile(file.ID))
		after, err := json.Marshal(item, json.Deterministic(true))
		require.NoError(t, err)
		require.Equal(t, before, after)
	}
}

func TestNewFileIDIsCXFIdentifier(t *testing.T) {
	first, err := NewFileID()
	require.NoError(t, err)
	second, err := NewFileID()
	require.NoError(t, err)
	require.NotEqual(t, first, second)
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(first)
	require.NoError(t, err)
	require.Len(t, decoded, 16)
}

func testFileCredential(t *testing.T, name, idSeed string) cxf.File {
	t.Helper()
	id, err := NewFileID()
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(idSeed))
	return cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            id,
		Name:          name,
		DecryptedSize: uint64(len(idSeed)),
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
}

func encodedCredential(t *testing.T, file cxf.File) string {
	t.Helper()
	data, err := json.Marshal(file, json.Deterministic(true))
	require.NoError(t, err)
	return string(data)
}
