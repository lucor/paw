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
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestItemHistorySurvivesPawExportImportAndRestore(t *testing.T) {
	ctx := context.Background()
	sourceStorage, sourceVault := newItemStoreVault(t)
	item, err := model.NewLogin("Version A", "synthetic-user-a", "synthetic-password-a", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Metadata.Favicon = model.NewFavicon("example.com", []byte("synthetic-icon"), "png")
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/future","value":"preserved"}`))
	creationAt, modifiedAt := uint64(10), uint64(11)
	item.CreationAt, item.ModifiedAt = &creationAt, &modifiedAt

	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 101))
	setSyntheticLoginVersion(t, item, "Version B", "synthetic-user-b", "synthetic-password-b")
	modifiedAt = 12
	item.ModifiedAt = &modifiedAt
	require.NoError(t, history.AddSnapshot(item, 202))
	setSyntheticLoginVersion(t, item, "Version C", "synthetic-user-c", "synthetic-password-c")
	modifiedAt = 13
	item.ModifiedAt = &modifiedAt
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, sourceStorage.StoreItem(sourceVault, item))

	beforeItem, err := os.ReadFile(itemPath(sourceStorage, sourceVault.Accounts[0].ID, item.ID))
	require.NoError(t, err)
	beforeVault, err := json.Marshal(sourceVault.Header, json.Deterministic(true))
	require.NoError(t, err)
	sourceHistory, err := item.History()
	require.NoError(t, err)
	var exportedItemJSON []byte

	for _, test := range []struct {
		name      string
		encrypted bool
	}{
		{
			name: "plaintext",
		},
		{
			name:      "age encrypted",
			encrypted: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var exported bytes.Buffer
			password := ""
			if test.encrypted {
				password = "synthetic-export-password"
				err = WriteEncryptedCXF(ctx, &exported, sourceStorage, sourceVault, password)
			} else {
				err = WriteCXF(ctx, &exported, sourceStorage, sourceVault)
			}
			require.NoError(t, err)

			cxfData := bytes.Clone(exported.Bytes())
			if test.encrypted {
				identity, err := age.NewScryptIdentity(password)
				require.NoError(t, err)
				identity.SetMaxWorkFactor(18)
				decrypted, err := age.Decrypt(bytes.NewReader(exported.Bytes()), identity)
				require.NoError(t, err)
				cxfData, err = io.ReadAll(decrypted)
				require.NoError(t, err)
			}

			document, err := cxf.Decode(bytes.NewReader(cxfData))
			require.NoError(t, err)
			require.Len(t, document.Accounts[0].Items, 1)
			for _, extension := range document.Accounts[0].Items[0].Extensions {
				var header struct {
					Name string `json:"name"`
				}
				require.NoError(t, json.Unmarshal(extension, &header))
				require.NotEqual(t, "paw.pm/item-hash", header.Name)
			}
			fullItemData, err := json.Marshal(document.Accounts[0].Items[0], json.Deterministic(true))
			require.NoError(t, err)
			if exportedItemJSON == nil {
				exportedItemJSON = bytes.Clone(fullItemData)
			} else {
				require.Equal(t, exportedItemJSON, fullItemData, "plaintext and encrypted exports contain the same item data")
			}
			var exportedItem model.Item
			require.NoError(t, json.Unmarshal(fullItemData, &exportedItem))
			exportedHistory, err := exportedItem.History()
			require.NoError(t, err)
			require.Equal(t, sourceHistory, exportedHistory)
			requireHistorySnapshotContent(t, &sourceHistory)

			backup, err := ReadPawBackup(ctx, bytes.NewReader(exported.Bytes()), password)
			require.NoError(t, err)
			importStorage := newTestFileStorage(t)
			importedVault, err := importStorage.Restore(ctx, backup, "Imported", "synthetic-master-password")
			require.NoError(t, err)
			importedItem, err := importStorage.LoadItem(importedVault, item.ID)
			require.NoError(t, err)
			require.Equal(t, item.Title, importedItem.Title)
			require.Contains(t, string(exportedItemJSON), "example.com/future")
			importedHistory, err := importedItem.History()
			require.NoError(t, err)
			require.Equal(t, sourceHistory, importedHistory, "import must not create or reorder snapshots")
			requireHistorySnapshotContent(t, &importedHistory)
			assertSummaryStateHash(t, importedVault.Accounts[0].Items[0], importedItem)
			assertStoredCXFItem(t, importStorage, importedVault, importedItem)

			restored, changed, err := importStorage.RestoreItem(importedVault, importedItem.ID, 1, &importedHistory.Entries[1], 200)
			require.NoError(t, err)
			require.True(t, changed)
			require.Equal(t, "Version B", restored.Title)
			assertLoginCredential(t, restored, "synthetic-user-b", "synthetic-password-b")
			restoredHistory, err := restored.History()
			require.NoError(t, err)
			require.Len(t, restoredHistory.Entries, 3)
			require.Equal(t, sourceHistory.Entries, restoredHistory.Entries[:2])
			require.Equal(t, "Version C", restoredHistory.Entries[2].Item.Title)
			assertStoredCXFItem(t, importStorage, importedVault, restored)
		})
	}

	afterItem, err := os.ReadFile(itemPath(sourceStorage, sourceVault.Accounts[0].ID, item.ID))
	require.NoError(t, err)
	afterVault, err := json.Marshal(sourceVault.Header, json.Deterministic(true))
	require.NoError(t, err)
	require.Equal(t, beforeItem, afterItem, "export must not rewrite the source item")
	require.Equal(t, beforeVault, afterVault, "export must not modify the source vault")
}

func TestRestoreRejectsInvalidItemHistory(t *testing.T) {
	vault, store, _ := exportFixture(t)
	var exported bytes.Buffer
	require.NoError(t, WriteCXF(context.Background(), &exported, store, vault))
	var document cxf.Header
	require.NoError(t, json.Unmarshal(exported.Bytes(), &document))
	document.Accounts[0].Items[0].Extensions = append(document.Accounts[0].Items[0].Extensions, jsontext.Value(`{"name":"paw.pm/history","schemaVersion":2,"entries":[]}`))
	var invalid bytes.Buffer
	require.NoError(t, cxf.Encode(&invalid, &document))
	_, err := ReadPawBackup(context.Background(), &invalid, "")
	require.ErrorIs(t, err, ErrUnsupportedBackup)
}

func setSyntheticLoginVersion(t *testing.T, item *model.Item, title, username, password string) {
	t.Helper()
	replacement, err := model.NewLogin(title, username, password, model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Title = title
	item.Credentials = append([]jsontext.Value(nil), replacement.Credentials...)
}

func requireHistorySnapshotContent(t *testing.T, history *model.History) {
	t.Helper()
	require.Len(t, history.Entries, 2)
	require.Equal(t, []uint64{101, 202}, []uint64{history.Entries[0].CapturedAt, history.Entries[1].CapturedAt})
	require.Equal(t, "Version A", history.Entries[0].Item.Title)
	require.Equal(t, "Version B", history.Entries[1].Item.Title)
	assertLoginCredential(t, &model.Item{Item: history.Entries[0].Item}, "synthetic-user-a", "synthetic-password-a")
	assertLoginCredential(t, &model.Item{Item: history.Entries[1].Item}, "synthetic-user-b", "synthetic-password-b")
	for _, entry := range history.Entries {
		data, err := json.Marshal(entry.Item, json.Deterministic(true))
		require.NoError(t, err)
		snapshot, err := decodeModelItem(data)
		require.NoError(t, err)
		require.Equal(t, "example.com", snapshot.Metadata.Favicon.Host)
		require.Equal(t, []byte("synthetic-icon"), snapshot.Metadata.Favicon.Data)
		require.Contains(t, string(data), "example.com/future")
		require.NotContains(t, string(data), "paw.pm/history")
		require.NotContains(t, string(data), "paw.pm/item-hash")
	}
}

func assertLoginCredential(t *testing.T, item *model.Item, username, password string) {
	t.Helper()
	var credential cxf.BasicAuth
	require.NoError(t, json.Unmarshal(item.Credentials[0], &credential))
	require.NotNil(t, credential.Username)
	require.NotNil(t, credential.Password)
	require.Equal(t, username, credential.Username.Value)
	require.Equal(t, password, credential.Password.Value)
}

func assertSummaryStateHash(t *testing.T, summary cxf.Item, item *model.Item) {
	t.Helper()
	stateHash, err := item.StateHash()
	require.NoError(t, err)
	var storedHash string
	for _, extension := range summary.Extensions {
		var hash struct {
			Name      string `json:"name"`
			Algorithm string `json:"algorithm"`
			Hash      string `json:"hash"`
		}
		require.NoError(t, json.Unmarshal(extension, &hash))
		if hash.Name == "paw.pm/item-hash" {
			require.Equal(t, "sha256", hash.Algorithm)
			storedHash = hash.Hash
		}
	}
	require.Equal(t, stateHash, storedHash)
}
