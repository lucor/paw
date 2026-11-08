// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func exportFixture(t *testing.T) (*model.Vault, *FileStorage, *model.Item) {
	t.Helper()
	store, vault := newItemStoreVault(t)
	item, err := model.NewLogin("Example", "me@example.com", "export-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	return vault, store, item
}

func TestExportCXFAndAgeDoNotExposeCredentials(t *testing.T) {
	vault, store, item := exportFixture(t)
	vault.Accounts[0].Collections = []cxf.Collection{{ID: "Y29sbGVjdGlvbg", Title: "Personal", Items: []cxf.LinkedItem{}, Extensions: []jsontext.Value{jsontext.Value(`{"name":"example.com/export","custom":"preserved"}`)}}}
	before, err := json.Marshal(vault.Header, json.Deterministic(true))
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
	encodedItem, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	actualItem, err := json.Marshal(document.Accounts[0].Items[0], json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(encodedItem), string(actualItem))
	require.NotContains(t, string(actualItem), "paw.pm/item-hash")
	require.NoError(t, WriteEncryptedCXF(context.Background(), &encrypted, store, vault, "dedicated-export-password"))
	require.True(t, bytes.HasPrefix(encrypted.Bytes(), []byte("age-encryption.org/v1\n")))
	require.NotContains(t, encrypted.String(), "export-secret")
	require.NotContains(t, encrypted.String(), "me@example.com")
	after, err := json.Marshal(vault.Header, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, before, after)
	vault.Accounts[0].Items = []cxf.Item{}
	plain.Reset()
	require.NoError(t, WriteCXF(context.Background(), &plain, store, vault))
	empty, err := cxf.Decode(&plain)
	require.NoError(t, err)
	require.Empty(t, empty.Accounts[0].Items)
}

func TestExportRejectsCurrentAndHistoricalAttachmentsBeforeWriting(t *testing.T) {
	for _, historical := range []bool{false, true} {
		t.Run(map[bool]string{false: "current reference", true: "history reference"}[historical], func(t *testing.T) {
			vault, store, item := exportFixture(t)
			_, err := store.AddAttachment(vault, item.ID, "file.bin", bytes.NewReader([]byte("payload")))
			require.NoError(t, err)
			if historical {
				files, err := store.ListAttachments(vault, item.ID)
				require.NoError(t, err)
				require.Len(t, files, 1)
				require.NoError(t, store.RemoveAttachment(vault, item.ID, files[0].ID))
			}
			var plaintext, encrypted bytes.Buffer
			require.ErrorContains(t, WriteCXF(context.Background(), &plaintext, store, vault), "cannot include attachment files")
			require.Empty(t, plaintext.Bytes())
			require.ErrorContains(t, WriteEncryptedCXF(context.Background(), &encrypted, store, vault, "password"), "cannot include attachment files")
			require.Empty(t, encrypted.Bytes())
		})
	}
}

func TestExportFailuresLeaveNoFiles(t *testing.T) {
	vault, store, _ := exportFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "credentials.cxf")
	ctx := context.Background()
	reader, writer := io.Pipe()
	require.NoError(t, reader.Close())
	require.Error(t, WriteCXF(ctx, writer, store, vault))
	require.NoError(t, writer.Close())
	reader, writer = io.Pipe()
	require.NoError(t, reader.Close())
	require.Error(t, WriteEncryptedCXF(ctx, writer, store, vault, "secret"))
	require.NoError(t, writer.Close())
	require.Error(t, ExportFile(ctx, store, vault, path, "", false))
	require.Error(t, ExportFile(ctx, store, vault, filepath.Join(dir, "missing", "file"), "", true))
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	require.ErrorIs(t, ExportFile(canceled, store, vault, path, "", true), context.Canceled)
	itemName := itemRelativePath(vault.Accounts[0].Items[0].ID, testSummaryStateHash(t, vault, vault.Accounts[0].Items[0].ID))
	vaultRoot, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	require.NoError(t, vaultRoot.Remove(itemName))
	require.NoError(t, vaultRoot.Close())
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

func TestExportCancellationBeforeCreationLeavesNoTemporaryFiles(t *testing.T) {
	for _, plaintext := range []bool{true, false} {
		vault, store, _ := exportFixture(t)
		dir := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		cancel()
		err := ExportFile(ctx, store, vault, filepath.Join(dir, "export"), "dedicated-password", plaintext)
		require.ErrorIs(t, err, context.Canceled)
		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, files)
	}
}

func TestWriteExportCancellationDuringWriting(t *testing.T) {
	for _, plaintext := range []bool{true, false} {
		t.Run(map[bool]string{true: "plaintext", false: "encrypted"}[plaintext], func(t *testing.T) {
			vault, store, _ := exportFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			done := make(chan error, 1)
			go func() {
				defer writer.Close()
				done <- WriteExport(ctx, writer, store, vault, "dedicated-password", plaintext)
			}()
			// Reading one byte leaves the first write blocked until after cancellation.
			var first [1]byte
			_, err := io.ReadFull(reader, first[:])
			require.NoError(t, err)
			cancel()
			_, err = io.Copy(io.Discard, reader)
			require.NoError(t, err)
			require.ErrorIs(t, <-done, context.Canceled)
		})
	}
}
