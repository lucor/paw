// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// TestStoreCXFItem checks creation and editing preserve full items and derived summaries.
func TestStoreCXFItem(t *testing.T) {
	for _, backend := range []string{"OS", "Fyne"} {
		t.Run(backend, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			key := vault.Key()
			writer := selectCXFTestBackend(t, storage, backend)
			vaultExtension := json.RawMessage(`{"name":"example.com/vault","value":"Preserved vault data"}`)
			vault.Accounts[0].Extensions = append(vault.Accounts[0].Extensions, vaultExtension)
			vault.Timestamp = 1
			item, err := model.NewNote("Original title", "Synthetic note secret")
			require.NoError(t, err)
			created := uint64(1700000000)
			item.CreationAt, item.ModifiedAt = &created, &created
			item.Extensions = append(item.Extensions, json.RawMessage(`{"name":"example.com/item","secret":"Private extension value"}`))
			item.Credentials = append(item.Credentials, json.RawMessage(`{"type":"example-credential","value":"Private credential value"}`))
			start := uint64(time.Now().Unix())
			require.NoError(t, writer.StoreItem(vault, item))
			require.GreaterOrEqual(t, vault.Timestamp, start)
			require.LessOrEqual(t, vault.Timestamp, uint64(time.Now().Unix()))
			require.Same(t, key, vault.Key())
			assertStoredCXFItem(t, storage, vault, item)

			other, err := model.NewNote("Other note", "Other content")
			require.NoError(t, err)
			require.NoError(t, writer.StoreItem(vault, other))
			otherSummary := vault.Accounts[0].Items[1]
			item.Title = "Renamed title"
			modified := created + 60
			item.ModifiedAt = &modified
			replacement, err := model.NewNote("Unused title", "Edited note secret")
			require.NoError(t, err)
			item.Credentials[0] = replacement.Credentials[0]
			require.NoError(t, writer.StoreItem(vault, item))
			require.Len(t, vault.Accounts[0].Items, 2)
			require.Equal(t, otherSummary, vault.Accounts[0].Items[1])
			require.Equal(t, created, *item.CreationAt)
			assertStoredCXFItem(t, storage, vault, item)
			decoded := readStoredCXFVault(t, storage, vault)
			require.Contains(t, decoded.Accounts[0].Extensions, vaultExtension)
			require.Equal(t, vault.Header, decoded.Header)
		})
	}
}

// TestStoreCXFItemFailures checks each backend's real file errors preserve write order and memory.
func TestStoreCXFItemFailures(t *testing.T) {
	for _, backend := range []string{"OS", "Fyne"} {
		for _, phase := range []string{"item", "vault"} {
			t.Run(backend+"/"+phase, func(t *testing.T) {
				storage, vault := newItemStoreVault(t)
				writer := selectCXFTestBackend(t, storage, backend)
				item, err := model.NewNote("Original", "Original content")
				require.NoError(t, err)
				require.NoError(t, writer.StoreItem(vault, item))
				vault.Metadata.VaultName = "Unsaved name"
				before, err := json.Marshal(&vault.Header)
				require.NoError(t, err)
				vaultFile := vaultPath(storage, vault.Accounts[0].ID)
				beforeFile, err := os.ReadFile(vaultFile)
				require.NoError(t, err)
				item.Title = "Edited title"
				failingPath := itemPath(storage, vault.Accounts[0].ID, item.ID)
				if phase == "vault" {
					failingPath = vaultFile
				}
				require.NoError(t, os.Remove(failingPath))
				require.NoError(t, os.Mkdir(failingPath, 0700))
				err = writer.StoreItem(vault, item)
				require.Error(t, err)
				if phase == "item" {
					require.ErrorContains(t, err, "could not store item")
					actual, err := os.ReadFile(vaultFile)
					require.NoError(t, err)
					require.Equal(t, beforeFile, actual)
				} else {
					require.ErrorContains(t, err, "after item was saved")
					actual := readStoredCXFItem(t, storage, vault, item.ID)
					require.Equal(t, item.Title, actual.Title)
				}
				after, err := json.Marshal(&vault.Header)
				require.NoError(t, err)
				require.Equal(t, before, after)
				require.Equal(t, "Unsaved name", vault.Metadata.VaultName)
			})
		}
	}
}

// TestStoreCXFItemValidatesBeforeWriting checks invalid inputs never open files.
func TestStoreCXFItemValidatesBeforeWriting(t *testing.T) {
	for _, invalid := range []string{"item", "vault", "key"} {
		t.Run(invalid, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			vaultFile := vaultPath(storage, vault.Accounts[0].ID)
			before, err := os.ReadFile(vaultFile)
			require.NoError(t, err)
			item, err := model.NewNote("Title", "Content")
			require.NoError(t, err)
			switch invalid {
			case "item":
				item.ID = "invalid ID"
			case "vault":
				vault.Accounts = nil
			case "key":
				vault = &model.Vault{Header: vault.Header, Metadata: vault.Metadata}
			}
			err = storage.storeItem(vault, item)
			require.Error(t, err)
			after, err := os.ReadFile(vaultFile)
			require.NoError(t, err)
			require.Equal(t, before, after)
			files, err := os.ReadDir(filepath.Dir(vaultFile))
			require.NoError(t, err)
			require.Len(t, files, 1)
		})
	}
}

// TestDeleteCXFItem checks both backends remove an item and its vault summary.
func TestDeleteCXFItem(t *testing.T) {
	for _, backend := range []string{"OS", "Fyne"} {
		t.Run(backend, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			writer := selectCXFTestBackend(t, storage, backend)
			first, err := model.NewNote("First", "First content")
			require.NoError(t, err)
			second, err := model.NewNote("Second", "Second content")
			require.NoError(t, err)
			require.NoError(t, writer.StoreItem(vault, first))
			require.NoError(t, writer.StoreItem(vault, second))
			key := vault.Key()
			start := uint64(time.Now().Unix())

			require.NoError(t, writer.DeleteItem(vault, first.ID))
			require.Same(t, key, vault.Key())
			require.GreaterOrEqual(t, vault.Timestamp, start)
			require.Len(t, vault.Accounts[0].Items, 1)
			require.Equal(t, second.ID, vault.Accounts[0].Items[0].ID)
			require.NoFileExists(t, itemPath(storage, vault.Accounts[0].ID, first.ID))
			require.FileExists(t, itemPath(storage, vault.Accounts[0].ID, second.ID))
			decoded := readStoredCXFVault(t, storage, vault)
			require.Len(t, decoded.Accounts[0].Items, 1)
			require.Equal(t, second.ID, decoded.Accounts[0].Items[0].ID)
		})
	}
}

// TestDeleteCXFItemFailures checks item-first deletion and memory update behavior.
func TestDeleteCXFItemFailures(t *testing.T) {
	for _, backend := range []string{"OS", "Fyne"} {
		for _, phase := range []string{"item", "vault"} {
			t.Run(backend+"/"+phase, func(t *testing.T) {
				storage, vault := newItemStoreVault(t)
				writer := selectCXFTestBackend(t, storage, backend)
				item, err := model.NewNote("Title", "Content")
				require.NoError(t, err)
				require.NoError(t, writer.StoreItem(vault, item))
				before, err := json.Marshal(&vault.Header)
				require.NoError(t, err)
				vaultFile := vaultPath(storage, vault.Accounts[0].ID)
				itemFile := itemPath(storage, vault.Accounts[0].ID, item.ID)
				failingPath := itemFile
				if phase == "vault" {
					failingPath = vaultFile
				}
				require.NoError(t, os.Remove(failingPath))
				require.NoError(t, os.Mkdir(failingPath, 0700))
				if phase == "item" {
					require.NoError(t, os.WriteFile(filepath.Join(failingPath, "blocker"), nil, 0600))
				}

				err = writer.DeleteItem(vault, item.ID)
				require.Error(t, err)
				if phase == "item" {
					require.ErrorContains(t, err, "could not delete item file")
					actual, readErr := json.Marshal(&readStoredCXFVault(t, storage, vault).Header)
					require.NoError(t, readErr)
					require.Equal(t, before, actual)
				} else {
					require.ErrorContains(t, err, "after item was deleted")
					require.NoFileExists(t, itemFile)
				}
				after, marshalErr := json.Marshal(&vault.Header)
				require.NoError(t, marshalErr)
				require.Equal(t, before, after)
			})
		}
	}
}

// TestDeleteCXFItemMissingSummary checks unknown items are rejected before file changes.
func TestDeleteCXFItemMissingSummary(t *testing.T) {
	for _, backend := range []string{"OS", "Fyne"} {
		t.Run(backend, func(t *testing.T) {
			storage, vault := newItemStoreVault(t)
			writer := selectCXFTestBackend(t, storage, backend)
			err := writer.DeleteItem(vault, "AAAAAAAAAAAAAAAAAAAAAA")
			require.ErrorContains(t, err, "is not in the vault")
			require.Empty(t, vault.Accounts[0].Items)
		})
	}
}

// TestPrepareItemVaultIsolated checks summary updates and serialization cannot mutate the source.
func TestPrepareItemVaultIsolated(t *testing.T) {
	storage, vault := newItemStoreVault(t)
	item, err := model.NewNote("Original", "Content")
	require.NoError(t, err)
	require.NoError(t, storage.storeItem(vault, item))
	vault.Metadata.VaultName = "Updated name"
	item.Title = "Updated title"
	before, err := json.Marshal(&vault.Header)
	require.NoError(t, err)
	updated, err := prepareItemVault(vault, item)
	require.NoError(t, err)
	_, err = json.Marshal(updated)
	require.NoError(t, err)
	after, err := json.Marshal(&vault.Header)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, "Updated title", updated.Accounts[0].Items[0].Title)
	require.Equal(t, "Original", vault.Accounts[0].Items[0].Title)
	require.Same(t, vault.Key(), updated.Key())
}

// TestEncryptFileFailures checks write and close errors reach both backend callers.
func TestEncryptFileFailures(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	writeErr := errors.New("write failed")
	closeErr := errors.New("close failed")
	for _, failure := range []string{"write", "close", "both"} {
		t.Run(failure, func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "encrypted")
			require.NoError(t, err)
			writer := &storeFaultFile{WriteCloser: file}
			if failure != "close" {
				writer.writeErr = writeErr
			}
			if failure != "write" {
				writer.closeErr = closeErr
			}
			err = encryptFile(key, writer, map[string]string{"note": "Synthetic content"})
			if writer.writeErr != nil {
				require.ErrorIs(t, err, writeErr)
			}
			if writer.closeErr != nil {
				require.ErrorIs(t, err, closeErr)
			}
			require.True(t, writer.closed)
			_, err = file.Write([]byte("closed"))
			require.ErrorIs(t, err, os.ErrClosed)
		})
	}
}

// newItemStoreVault creates an encrypted vault without password generation overhead.
func newItemStoreVault(t *testing.T) (*OSStorage, *model.Vault) {
	t.Helper()
	backend, err := NewOSStorageRooted(t.TempDir())
	require.NoError(t, err)
	storage := backend.(*OSStorage)
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	vault, err := model.NewVault(key, "Personal")
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(vaultRootPath(storage, vault.Accounts[0].ID), 0700))
	require.NoError(t, storage.storeEncrypted(vaultPath(storage, vault.Accounts[0].ID), key, vault))
	return storage, vault
}

// assertStoredCXFItem checks full credentials stay in the item and out of its summary.
func assertStoredCXFItem(t *testing.T, storage *OSStorage, vault *model.Vault, item *model.Item) {
	t.Helper()
	actual := readStoredCXFItem(t, storage, vault, item.ID)
	require.Equal(t, item.Item, actual.Item)
	require.Equal(t, item.Metadata, actual.Metadata)
	decoded := readStoredCXFVault(t, storage, vault)
	summary, err := item.Summary()
	require.NoError(t, err)
	require.Equal(t, *summary, decoded.Accounts[0].Items[0])
	require.Empty(t, decoded.Accounts[0].Items[0].Credentials)
	data, err := json.Marshal(&decoded.Header)
	require.NoError(t, err)
	require.NotContains(t, string(data), "Synthetic note secret")
	require.NotContains(t, string(data), "Private extension value")
	require.NotContains(t, string(data), "Private credential value")
}

// readStoredCXFItem decrypts an item file using the same full-stream reader as storage.
func readStoredCXFItem(t *testing.T, storage *OSStorage, vault *model.Vault, id string) *model.Item {
	t.Helper()
	data, err := os.ReadFile(itemPath(storage, vault.Accounts[0].ID, id))
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("age-encryption.org/v1")))
	item, err := decryptItem(vault.Key(), bytes.NewReader(data), id)
	require.NoError(t, err)
	return item
}

// readStoredCXFVault decrypts the saved vault without reading its item files.
func readStoredCXFVault(t *testing.T, storage *OSStorage, vault *model.Vault) *model.Vault {
	t.Helper()
	data, err := os.ReadFile(vaultPath(storage, vault.Accounts[0].ID))
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(data, []byte("age-encryption.org/v1")))
	decoded, err := decryptVault(vault.Key(), bytes.NewReader(data))
	require.NoError(t, err)
	return decoded
}

type storeFaultFile struct {
	io.WriteCloser
	writeErr error
	closeErr error
	closed   bool
}

// Write fails at the selected item or vault write boundary.
func (file *storeFaultFile) Write(data []byte) (int, error) {
	if file.writeErr != nil {
		return 0, file.writeErr
	}
	return file.WriteCloser.Write(data)
}

// Close closes the real file and reports any configured close failure.
func (file *storeFaultFile) Close() error {
	file.closed = true
	return errors.Join(file.WriteCloser.Close(), file.closeErr)
}
