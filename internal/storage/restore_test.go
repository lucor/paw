// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"io"
	"os"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestRestorePreservesItemsAndCreatesCopy(t *testing.T) {
	ctx := context.Background()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	source, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	item, err := model.NewNote("Example", "Synthetic note content")
	require.NoError(t, err)
	itemData, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	var exportedItem cxf.Item
	require.NoError(t, json.Unmarshal(itemData, &exportedItem))
	accountID := source.Accounts[0].ID
	source.Accounts[0].Items = []cxf.Item{exportedItem}
	source.Accounts[0].Collections = []cxf.Collection{{ID: "Y29sbGVjdGlvbg", Title: "Personal", Items: []cxf.LinkedItem{{Item: item.ID, Account: &accountID}}}}
	var encoded bytes.Buffer
	require.NoError(t, cxf.Encode(&encoded, &source.Header))
	backup := &PawBackup{data: encoded.Bytes(), Name: "Personal", CreationAt: source.Metadata.CreationAt, Collections: 1, Credentials: 1, AccountID: accountID}
	store := newTestFileStorage(t)
	restored, err := store.Restore(ctx, backup, "Personal", "new-master")
	require.NoError(t, err)
	require.Equal(t, accountID, restored.Accounts[0].ID)
	require.Equal(t, source.Metadata.CreationAt, restored.Metadata.CreationAt)
	_, err = store.LoadVault(accountID, source.Key())
	require.Error(t, err, "the original encryption key must not unlock the restored vault")
	loaded, err := store.LoadVault(accountID, restored.Key())
	require.NoError(t, err)
	restoredItem, err := store.LoadItem(loaded, item.ID)
	require.NoError(t, err)
	itemHistory, err := restoredItem.History()
	require.NoError(t, err)
	require.Empty(t, itemHistory.Entries)
	assertSummaryStateHash(t, loaded.Accounts[0].Items[0], restoredItem)
	restoredItemData, err := json.Marshal(restoredItem, json.Deterministic(true))
	require.NoError(t, err)
	expectedItemData, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(expectedItemData), string(restoredItemData))
	copy, err := store.Restore(ctx, backup, "Personal copy", "copy-master")
	require.NoError(t, err)
	require.NotEqual(t, accountID, copy.Accounts[0].ID)
	unchanged, err := store.LoadVault(accountID, restored.Key())
	require.NoError(t, err)
	require.Equal(t, "Personal", unchanged.Metadata.VaultName)
	require.Greater(t, *copy.Metadata.CreationAt, *source.Metadata.CreationAt)
	require.Equal(t, item.ID, copy.Accounts[0].Items[0].ID)
	require.Equal(t, source.Accounts[0].Collections[0].ID, copy.Accounts[0].Collections[0].ID)
	require.Equal(t, copy.Accounts[0].ID, *copy.Accounts[0].Collections[0].Items[0].Account)
	copyItem, err := store.LoadItem(copy, item.ID)
	require.NoError(t, err)
	copyData, err := json.Marshal(copyItem, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(expectedItemData), string(copyData))
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Len(t, vaults, 2)
}

type cancelReservedRestoreContext struct {
	context.Context
	cancel context.CancelFunc
	root   string
}

// Err cancels as soon as restore has reserved its unpublished vault directory.
func (c cancelReservedRestoreContext) Err() error {
	entries, _ := os.ReadDir(c.root)
	if len(entries) > 0 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestRestoreCancellationRemovesUnpublishedVault(t *testing.T) {
	source, loader, _ := exportFixture(t)
	var data bytes.Buffer
	require.NoError(t, WriteCXF(context.Background(), &data, loader, source))
	backup, err := ReadPawBackup(context.Background(), &data, "")
	require.NoError(t, err)
	store := newTestFileStorage(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = store.Restore(cancelReservedRestoreContext{ctx, cancel, storageRootPath(store)}, backup, "Restored", "master")
	require.ErrorIs(t, err, context.Canceled)
	entries, err := os.ReadDir(storageRootPath(store))
	require.NoError(t, err)
	require.Empty(t, entries)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
}

func TestRestoreRejectsIncompatibleVersionsAndOversizedInput(t *testing.T) {
	source, loader, _ := exportFixture(t)
	var data bytes.Buffer
	require.NoError(t, WriteCXF(context.Background(), &data, loader, source))
	var document cxf.Header
	require.NoError(t, json.Unmarshal(data.Bytes(), &document))
	document.Version.Major = 2
	wrongVersion, err := json.Marshal(document, json.Deterministic(true))
	require.NoError(t, err)
	wrongPawVersion := bytes.ReplaceAll(data.Bytes(), []byte(`"schemaVersion":1`), []byte(`"schemaVersion":99`))
	require.NotEqual(t, data.Bytes(), wrongPawVersion)
	for _, invalid := range [][]byte{wrongVersion, wrongPawVersion, []byte(`[]`)} {
		_, err := ReadPawBackup(context.Background(), bytes.NewReader(invalid), "")
		require.ErrorIs(t, err, ErrUnsupportedBackup)
	}
	_, err = ReadPawBackup(context.Background(), strings.NewReader(strings.Repeat("x", maxBackupBytes+1)), "")
	require.ErrorContains(t, err, "64 MiB")
}

func TestRestoreRejectsUnsupportedAndUnauthenticatedInput(t *testing.T) {
	source, loader, _ := exportFixture(t)
	var plain, encrypted bytes.Buffer
	ctx := context.Background()
	require.NoError(t, WriteCXF(ctx, &plain, loader, source))
	encrypted.Write(writeEncryptedBackup(t, plain.Bytes(), "backup-secret"))
	_, err := ReadPawBackup(ctx, bytes.NewReader(encrypted.Bytes()), "")
	require.ErrorIs(t, err, ErrBackupPasswordRequired)
	_, err = ReadPawBackup(ctx, bytes.NewReader(encrypted.Bytes()), "wrong")
	require.Error(t, err)
	_, err = ReadPawBackup(ctx, bytes.NewReader(encrypted.Bytes()[:encrypted.Len()-1]), "backup-secret")
	require.Error(t, err)
	changed := bytes.Clone(encrypted.Bytes())
	changed[len(changed)-1] ^= 1
	_, err = ReadPawBackup(ctx, bytes.NewReader(changed), "backup-secret")
	require.Error(t, err)
	var generic cxf.Header
	require.NoError(t, json.Unmarshal(plain.Bytes(), &generic))
	generic.Accounts[0].Extensions = nil
	data, err := json.Marshal(generic, json.Deterministic(true))
	require.NoError(t, err)
	_, err = ReadPawBackup(ctx, bytes.NewReader(data), "")
	require.ErrorIs(t, err, ErrUnsupportedBackup)
	_, err = ReadPawBackup(ctx, bytes.NewBufferString("invalid"), "")
	require.ErrorIs(t, err, ErrUnsupportedBackup)
	backup, err := ReadPawBackup(ctx, bytes.NewReader(plain.Bytes()), "")
	require.NoError(t, err)
	store := newTestFileStorage(t)
	_, err = store.Restore(ctx, backup, "Personal", "")
	require.Error(t, err)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = store.Restore(canceled, backup, "Personal", "master")
	require.ErrorIs(t, err, context.Canceled)
	backup.data = []byte("invalid")
	_, err = store.Restore(ctx, backup, "Personal", "master")
	require.ErrorIs(t, err, ErrUnsupportedBackup)
	vaults, err := store.Vaults()
	require.NoError(t, err)
	require.Empty(t, vaults)
	entries, err := os.ReadDir(storageRootPath(store))
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestRestoreDetectsLateAgeIntegrityFailure(t *testing.T) {
	source, loader, _ := exportFixture(t)
	var plain bytes.Buffer
	require.NoError(t, WriteCXF(context.Background(), &plain, loader, source))
	// Put authenticated JSON before a later damaged age chunk.
	payload := append(plain.Bytes(), bytes.Repeat([]byte(" "), 128<<10)...)
	var encoded bytes.Buffer
	recipient, err := age.NewScryptRecipient("secret")
	require.NoError(t, err)
	recipient.SetWorkFactor(1)
	w, err := age.Encrypt(&encoded, recipient)
	require.NoError(t, err)
	_, err = io.Copy(w, bytes.NewReader(payload))
	require.NoError(t, err)
	require.NoError(t, w.Close())
	data := encoded.Bytes()
	data[len(data)-1] ^= 1
	_, err = ReadPawBackup(context.Background(), bytes.NewReader(data), "secret")
	require.Error(t, err)
}

// writeEncryptedBackup creates a cheap age fixture for backup reader tests.
func writeEncryptedBackup(t *testing.T, data []byte, password string) []byte {
	t.Helper()
	recipient, err := age.NewScryptRecipient(password)
	require.NoError(t, err)
	recipient.SetWorkFactor(1)
	var encrypted bytes.Buffer
	writer, err := age.Encrypt(&encrypted, recipient)
	require.NoError(t, err)
	_, err = writer.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return encrypted.Bytes()
}
