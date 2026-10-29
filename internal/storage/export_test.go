// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func exportFixture(t *testing.T) (*model.Vault, *StorageMock, *model.Item) {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	item, err := model.NewLogin("Example", "me@example.com", "export-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	summary, err := item.Summary()
	require.NoError(t, err)
	vault.Accounts[0].Items = append(vault.Accounts[0].Items, *summary)
	store := &StorageMock{}
	store.OnLoadItem = func(_ *model.Vault, id string) (*model.Item, error) { require.Equal(t, item.ID, id); return item, nil }
	return vault, store, item
}

func TestExportCXFAndAgePreserveCredentials(t *testing.T) {
	vault, store, item := exportFixture(t)
	vault.Accounts[0].Collections = []cxf.Collection{{ID: "Y29sbGVjdGlvbg", Title: "Personal", Items: []cxf.LinkedItem{}, Extensions: []json.RawMessage{json.RawMessage(`{"name":"example.com/export","custom":"preserved"}`)}}}
	before, err := json.Marshal(vault.Header)
	require.NoError(t, err)
	var plain, encrypted bytes.Buffer
	require.NoError(t, WriteCXF(context.Background(), &plain, store, vault))
	document, err := cxf.Decode(bytes.NewReader(plain.Bytes()))
	require.NoError(t, err)
	require.Equal(t, vault.Accounts[0].ID, document.Accounts[0].ID)
	require.Equal(t, vault.Accounts[0].Collections, document.Accounts[0].Collections)
	require.Len(t, document.Accounts[0].Extensions, len(vault.Accounts[0].Extensions))
	for i, extension := range vault.Accounts[0].Extensions {
		require.JSONEq(t, string(extension), string(document.Accounts[0].Extensions[i]))
	}
	require.Equal(t, item.ID, document.Accounts[0].Items[0].ID)
	encodedItem, err := json.Marshal(item)
	require.NoError(t, err)
	actualItem, err := json.Marshal(document.Accounts[0].Items[0])
	require.NoError(t, err)
	require.JSONEq(t, string(encodedItem), string(actualItem))
	require.NoError(t, WriteEncryptedCXF(context.Background(), &encrypted, store, vault, "dedicated-export-password"))
	require.NotContains(t, encrypted.String(), "export-secret")
	require.NotContains(t, encrypted.String(), "me@example.com")
	identity, err := age.NewScryptIdentity("dedicated-export-password")
	require.NoError(t, err)
	reader, err := age.Decrypt(bytes.NewReader(encrypted.Bytes()), identity)
	require.NoError(t, err)
	decrypted, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, plain.Bytes(), decrypted)
	wrong, err := age.NewScryptIdentity("incorrect")
	require.NoError(t, err)
	_, err = age.Decrypt(bytes.NewReader(encrypted.Bytes()), wrong)
	require.Error(t, err)
	after, err := json.Marshal(vault.Header)
	require.NoError(t, err)
	require.Equal(t, before, after)
	vault.Accounts[0].Items = []cxf.Item{}
	plain.Reset()
	require.NoError(t, WriteCXF(context.Background(), &plain, store, vault))
	empty, err := cxf.Decode(&plain)
	require.NoError(t, err)
	require.Empty(t, empty.Accounts[0].Items)
}

type failingExportWriter struct{}

func (failingExportWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }

func TestExportFailuresLeaveNoFiles(t *testing.T) {
	vault, store, _ := exportFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.cxf")
	ctx := context.Background()
	require.Error(t, WriteCXF(ctx, failingExportWriter{}, store, vault))
	require.Error(t, WriteEncryptedCXF(ctx, failingExportWriter{}, store, vault, "secret"))
	require.Error(t, ExportFile(ctx, store, vault, path, "", false))
	require.Error(t, ExportFile(ctx, store, vault, filepath.Join(dir, "missing", "file"), "", true))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, ExportFile(canceled, store, vault, path, "", true), context.Canceled)
	store.OnLoadItem = func(*model.Vault, string) (*model.Item, error) { return nil, errors.New("load failed") }
	require.Error(t, ExportFile(ctx, store, vault, path, "", true))
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, files)
	vault, store, _ = exportFixture(t)
	require.NoError(t, ExportFile(ctx, store, vault, path, "", true))
	info, err := os.Stat(path)
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Error(t, ExportFile(ctx, store, vault, path, "", true))
	after, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, data, after)
}

func TestExportCancellationDuringLoadingRemovesTemporaryFiles(t *testing.T) {
	for _, plaintext := range []bool{true, false} {
		vault, store, item := exportFixture(t)
		dir := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		store.OnLoadItem = func(*model.Vault, string) (*model.Item, error) {
			if !plaintext {
				files, err := os.ReadDir(dir)
				require.NoError(t, err)
				for _, file := range files {
					data, err := os.ReadFile(filepath.Join(dir, file.Name()))
					require.NoError(t, err)
					require.NotContains(t, string(data), "export-secret")
					require.NotContains(t, string(data), `"accounts"`)
				}
			}
			cancel()
			return item, nil
		}
		err := ExportFile(ctx, store, vault, filepath.Join(dir, "export"), "dedicated-password", plaintext)
		require.ErrorIs(t, err, context.Canceled)
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, files)
	}
}
