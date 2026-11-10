// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestCreateVaultStoresHybridIdentityAndEncryptedData(t *testing.T) {
	store := newTestFileStorage(t)
	vault, err := store.CreateVault("Personal", "password")
	require.NoError(t, err)
	id := vault.Accounts[0].ID
	require.FileExists(t, keyPath(store, id))
	require.NoFileExists(t, legacyKeyPath(store, id))
	key, err := store.LoadVaultKey(id, "password")
	require.NoError(t, err)
	_, err = age.ParseHybridIdentity(key.String())
	require.NoError(t, err)

	item, err := model.NewNote("Encrypted note", "private content")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 123))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	file, err := store.AddAttachment(vault, item.ID, "document.txt", strings.NewReader("private attachment"))
	require.NoError(t, err)
	item, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	history, err = item.History()
	require.NoError(t, err)

	loadedVault, err := store.LoadVault(id, key)
	require.NoError(t, err)
	loadedItem, err := store.LoadItem(loadedVault, item.ID)
	require.NoError(t, err)
	require.Equal(t, item.Title, loadedItem.Title)
	loadedHistory, err := loadedItem.History()
	require.NoError(t, err)
	require.Equal(t, history, loadedHistory)
	require.NoError(t, store.SaveAttachment(loadedVault, item.ID, file.ID, attachmentDestination(t)))
}

func TestLoadVaultKeyMigratesLegacyVaultAndPreservesData(t *testing.T) {
	const password = "migration password"
	store, vault, item, history, attachmentID := createLegacyIdentityVault(t, password)
	id := vault.Accounts[0].ID
	vaultBefore, err := os.ReadFile(vaultPath(store, id))
	require.NoError(t, err)
	metadataBefore, err := os.ReadFile(vaultMetadataPath(store, id))
	require.NoError(t, err)
	legacyKeyBefore, err := os.ReadFile(legacyKeyPath(store, id))
	require.NoError(t, err)

	key, err := store.LoadVaultKey(id, password)
	require.NoError(t, err)
	require.True(t, key.Migrated())
	_, err = age.ParseHybridIdentity(key.String())
	require.NoError(t, err)
	require.FileExists(t, keyPath(store, id))
	require.NoFileExists(t, legacyKeyPath(store, id))
	require.NoDirExists(t, vaultRootPath(store, id)+identityBackupSuffix)

	loaded, err := store.LoadVault(id, key)
	require.NoError(t, err)
	require.Equal(t, vault.Timestamp, loaded.Timestamp)
	require.Equal(t, vault.Metadata, loaded.Metadata)
	loadedItem, err := store.LoadItem(loaded, item.ID)
	require.NoError(t, err)
	require.Equal(t, item.ID, loadedItem.ID)
	require.Equal(t, item.Title, loadedItem.Title)
	require.Equal(t, item.Credentials, loadedItem.Credentials)
	loadedHistory, err := loadedItem.History()
	require.NoError(t, err)
	require.Equal(t, history, loadedHistory)
	destination := attachmentDestination(t)
	require.NoError(t, store.SaveAttachment(loaded, item.ID, attachmentID, destination))
	attachment, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, "private attachment", string(attachment))

	vaultAfter, err := os.ReadFile(vaultPath(store, id))
	require.NoError(t, err)
	metadataAfter, err := os.ReadFile(vaultMetadataPath(store, id))
	require.NoError(t, err)
	require.NotEqual(t, vaultBefore, vaultAfter)
	require.Equal(t, metadataBefore, metadataAfter)
	require.NotEqual(t, legacyKeyBefore, readMigrationTestFile(t, keyPath(store, id)))

	reopenedKey, err := store.LoadVaultKey(id, password)
	require.NoError(t, err)
	require.False(t, reopenedKey.Migrated())
}

func TestLegacyUnlockWrongPasswordDoesNotChangeVault(t *testing.T) {
	store, vault, _, _, _ := createLegacyIdentityVault(t, "correct password")
	id := vault.Accounts[0].ID
	before := snapshotVaultFiles(t, vaultRootPath(store, id))
	_, err := store.LoadVaultKey(id, "wrong password")
	require.Error(t, err)
	require.Equal(t, before, snapshotVaultFiles(t, vaultRootPath(store, id)))
	require.NoDirExists(t, vaultRootPath(store, id)+identityStageSuffix)
}

func TestLegacyUnlockRecoversInterruptedIdentityMigration(t *testing.T) {
	for _, testCase := range []struct {
		name         string
		corruptStage bool
	}{
		{name: "before activation"},
		{name: "during activation"},
		{name: "invalid staged data", corruptStage: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			const password = "recovery password"
			store, vault, _, _, _ := createLegacyIdentityVault(t, password)
			id := vault.Accounts[0].ID
			active := vaultDirectory(id)
			stage := active + identityStageSuffix
			backup := active + identityBackupSuffix
			oldKey, err := loadIdentityAt(store, active, keyFileName, password, false)
			require.NoError(t, err)
			require.NoError(t, store.prepareIdentityStage(active, stage, password, oldKey))
			if testCase.corruptStage {
				require.NoError(t, os.WriteFile(filepath.Join(store.Root(), stage, vaultFileName), []byte("corrupt staged file"), 0o600))
			}
			if testCase.name != "before activation" {
				require.NoError(t, store.root.Rename(active, backup))
				listed, err := store.Vaults()
				require.NoError(t, err)
				require.Len(t, listed, 1, "interrupted vault remains available for unlock")
				require.Equal(t, id, listed[0].ID)
			}

			key, err := store.LoadVaultKey(id, password)
			require.NoError(t, err)
			require.True(t, key.Migrated())
			loaded, err := store.LoadVault(id, key)
			require.NoError(t, err)
			require.Len(t, loaded.Accounts[0].Items, 1)
			require.NoDirExists(t, filepath.Join(store.Root(), stage))
			require.NoDirExists(t, filepath.Join(store.Root(), backup))
		})
	}
}

func TestLegacyUnlockRejectsCorruptAndAmbiguousVaults(t *testing.T) {
	t.Run("corrupt encrypted data", func(t *testing.T) {
		store, vault, _, _, _ := createLegacyIdentityVault(t, "correct password")
		id := vault.Accounts[0].ID
		beforeKey := readMigrationTestFile(t, legacyKeyPath(store, id))
		require.NoError(t, os.WriteFile(vaultPath(store, id), []byte("corrupt"), 0o600))
		_, err := store.LoadVaultKey(id, "correct password")
		require.Error(t, err)
		require.Equal(t, beforeKey, readMigrationTestFile(t, legacyKeyPath(store, id)))
		require.NoDirExists(t, vaultRootPath(store, id)+identityBackupSuffix)
	})

	t.Run("both identity paths", func(t *testing.T) {
		store, vault, _, _, _ := createLegacyIdentityVault(t, "correct password")
		id := vault.Accounts[0].ID
		require.NoError(t, os.MkdirAll(filepath.Dir(keyPath(store, id)), 0o700))
		require.NoError(t, os.WriteFile(keyPath(store, id), []byte("unexpected identity"), 0o600))
		_, err := store.LoadVaultKey(id, "correct password")
		require.ErrorContains(t, err, "both legacy and post-quantum identities")
		require.FileExists(t, legacyKeyPath(store, id))
	})
}

func createLegacyIdentityVault(t *testing.T, password string) (*FileStorage, *model.Vault, *model.Item, model.History, string) {
	t.Helper()
	store := newTestFileStorage(t)
	vault, err := store.CreateVault("Migration", password)
	require.NoError(t, err)
	item, err := model.NewNote("History note", "current secret")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	history, err := item.History()
	require.NoError(t, err)
	previous := *item
	previous.Credentials = append(previous.Credentials, item.Credentials...)
	require.NoError(t, history.AddSnapshot(&previous, 123))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	attachment, err := store.AddAttachment(vault, item.ID, "document.txt", strings.NewReader("private attachment"))
	require.NoError(t, err)
	item, err = store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	history, err = item.History()
	require.NoError(t, err)

	identity, err := age.GenerateX25519Identity()
	require.NoError(t, err)
	identityJSON, err := json.Marshal(identity.String())
	require.NoError(t, err)
	var legacyKey paw.Key
	require.NoError(t, legacyKey.UnmarshalJSON(identityJSON))
	root, err := store.openVaultRoot(vault.Accounts[0].ID)
	require.NoError(t, err)
	entries, err := os.ReadDir(vaultRootPath(store, vault.Accounts[0].ID))
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.IsDir() {
			if entry.Name() == identitiesDirectoryName {
				continue
			}
			if err := reencryptLegacyTestTree(root, root, entry.Name(), vault.Key(), &legacyKey); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".age") || entry.Name() == syncIdentityFile {
			continue
		}
		if err := reencryptLegacyTestFile(root, root, entry.Name(), vault.Key(), &legacyKey); err != nil {
			t.Fatal(err)
		}
	}
	var wrapped bytes.Buffer
	require.NoError(t, legacyKey.WriteWrapped(&wrapped, password))
	require.NoError(t, root.WriteFile(keyFileName, wrapped.Bytes(), 0o600))
	require.NoError(t, root.Close())
	require.NoError(t, os.RemoveAll(filepath.Join(vaultRootPath(store, vault.Accounts[0].ID), identitiesDirectoryName)))
	return store, vault, item, history, attachment.ID
}

func reencryptLegacyTestTree(root *os.Root, _ *os.Root, dir string, from, to *paw.Key) error {
	entries, err := os.ReadDir(filepath.Join(root.Name(), dir))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if err := reencryptLegacyTestTree(root, root, path, from, to); err != nil {
				return err
			}
		} else if strings.HasSuffix(entry.Name(), ".age") {
			if err := reencryptLegacyTestFile(root, root, path, from, to); err != nil {
				return err
			}
		}
	}
	return nil
}

func reencryptLegacyTestFile(root *os.Root, _ *os.Root, path string, from, to *paw.Key) error {
	file, err := root.Open(path)
	if err != nil {
		return err
	}
	plain, err := from.Decrypt(file)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(plain)
	if err != nil {
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	var encrypted bytes.Buffer
	writer, err := to.Encrypt(&encrypted)
	if err != nil {
		return err
	}
	if _, err := writer.Write(data); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return root.WriteFile(path, encrypted.Bytes(), 0o600)
}

func snapshotVaultFiles(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		files[relative], err = os.ReadFile(path)
		return err
	})
	require.NoError(t, err)
	return files
}

func readMigrationTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func attachmentDestination(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return filepath.Join(dir, "attachment.txt")
}
