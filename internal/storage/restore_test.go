// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"filippo.io/age"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
)

func TestPawRestoreRoundTripAndCopies(t *testing.T) {
	for _, encrypted := range []bool{false, true} {
		for _, backend := range []string{"os", "fyne"} {
			t.Run(backend+map[bool]string{false: "/plaintext", true: "/age"}[encrypted], func(t *testing.T) {
				ctx := context.Background()
				source, loader, item := exportFixture(t)
				originalCreation := uint64(1234567890)
				source.Metadata.CreationAt = &originalCreation
				accountID := source.Accounts[0].ID
				source.Accounts[0].Collections = []cxf.Collection{{ID: "Y29sbGVjdGlvbg", Title: "Personal", Items: []cxf.LinkedItem{{Item: item.ID, Account: &accountID}}}}
				var plain, encoded bytes.Buffer
				require.NoError(t, WriteCXF(ctx, &plain, loader, source))
				password := ""
				if encrypted {
					password = "export-password"
					require.NoError(t, WriteEncryptedCXF(ctx, &encoded, loader, source, password))
				} else {
					encoded.Write(plain.Bytes())
				}
				backup, err := ReadPawBackup(ctx, bytes.NewReader(encoded.Bytes()), password)
				require.NoError(t, err)
				require.Equal(t, "Personal", backup.Name)
				require.Equal(t, 1, backup.Collections)
				require.Equal(t, 1, backup.Credentials)
				var store Storage
				if backend == "os" {
					store, err = NewOSStorageRooted(t.TempDir())
				} else {
					app := test.NewTempApp(t)
					store, err = NewFyneStorage(rootedFyneStorage{Storage: app.Storage(), root: fynestorage.NewFileURI(t.TempDir())})
				}
				require.NoError(t, err)
				collision, err := BackupCollision(store, backup)
				require.NoError(t, err)
				require.False(t, collision)
				restored, err := RestorePawBackup(ctx, store, backup, "Personal", "new-master")
				require.NoError(t, err)
				require.Equal(t, accountID, restored.Accounts[0].ID)
				require.Equal(t, source.Metadata.CreationAt, restored.Metadata.CreationAt)
				key, err := store.LoadVaultKey(accountID, "new-master")
				require.NoError(t, err)
				_, err = store.LoadVaultKey(accountID, "export-password")
				require.Error(t, err)
				_, err = store.LoadVault(accountID, source.Key())
				require.Error(t, err, "the original encryption key must not unlock the restored vault")
				loaded, err := store.LoadVault(accountID, key)
				require.NoError(t, err)
				var actual bytes.Buffer
				require.NoError(t, WriteCXF(ctx, &actual, store, loaded))
				require.JSONEq(t, plain.String(), actual.String())
				copy, err := RestorePawBackup(ctx, store, backup, "Personal copy", "copy-master")
				require.NoError(t, err)
				require.NotEqual(t, accountID, copy.Accounts[0].ID)
				require.Greater(t, *copy.Metadata.CreationAt, originalCreation)
				require.Equal(t, item.ID, copy.Accounts[0].Items[0].ID)
				require.Equal(t, source.Accounts[0].Collections[0].ID, copy.Accounts[0].Collections[0].ID)
				require.Equal(t, copy.Accounts[0].ID, *copy.Accounts[0].Collections[0].Items[0].Account)
				originalItem, err := store.LoadItem(loaded, item.ID)
				require.NoError(t, err)
				copyItem, err := store.LoadItem(copy, item.ID)
				require.NoError(t, err)
				originalData, err := json.Marshal(originalItem)
				require.NoError(t, err)
				copyData, err := json.Marshal(copyItem)
				require.NoError(t, err)
				require.JSONEq(t, string(originalData), string(copyData))
				vaults, err := store.Vaults()
				require.NoError(t, err)
				require.Len(t, vaults, 2)
			})
		}
	}
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
	store, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err = RestorePawBackup(cancelReservedRestoreContext{ctx, cancel, storageRootPath(store)}, store, backup, "Restored", "master")
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
	wrongVersion, err := json.Marshal(document)
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
	require.NoError(t, WriteEncryptedCXF(ctx, &encrypted, loader, source, "backup-secret"))
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
	data, err := json.Marshal(generic)
	require.NoError(t, err)
	_, err = ReadPawBackup(ctx, bytes.NewReader(data), "")
	require.ErrorIs(t, err, ErrUnsupportedBackup)
	_, err = ReadPawBackup(ctx, bytes.NewBufferString("invalid"), "")
	require.ErrorIs(t, err, ErrUnsupportedBackup)
	backup, err := ReadPawBackup(ctx, bytes.NewReader(plain.Bytes()), "")
	require.NoError(t, err)
	store, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	_, err = RestorePawBackup(ctx, store, backup, "Personal", "")
	require.Error(t, err)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = RestorePawBackup(canceled, store, backup, "Personal", "master")
	require.ErrorIs(t, err, context.Canceled)
	backup.data = []byte("invalid")
	_, err = RestorePawBackup(ctx, store, backup, "Personal", "master")
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
