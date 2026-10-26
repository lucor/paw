// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	fynestorage "fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/test"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"lucor.dev/paw/internal/migration"
	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

// TestLoadVaultMigratesLegacyVaultAndPreservesKeyFile checks both backends migrate a complete legacy vault.
func TestLoadVaultMigratesLegacyVaultAndPreservesKeyFile(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range []struct {
		name    string
		backend func(t *testing.T, root string) Storage
	}{
		{
			name: "OS",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewOSStorageRooted(root)
				require.NoError(t, err)
				return backend
			},
		},
		{
			name: "Fyne",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewFyneStorage(rootedFyneStorage{
					Storage: app.Storage(), root: fynestorage.NewFileURI(root),
				})
				require.NoError(t, err)
				return backend
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			root := t.TempDir()
			s := testCase.backend(t, root)
			key, legacy, _, keyData, _ := seedLegacyMigrationVault(t, s, true)
			legacyID := legacy.Name

			migrated, err := s.LoadVault(legacyID, key)
			require.NoError(t, err)
			require.NotEqual(t, legacyID, migrated.Accounts[0].ID)
			newID := migrated.Accounts[0].ID
			require.NoDirExists(t, legacyVaultRootPath(s, legacyID))
			require.FileExists(t, keyPath(s, newID))
			actualKeyData, err := os.ReadFile(keyPath(s, newID))
			require.NoError(t, err)
			require.Equal(t, keyData, actualKeyData)
			itemSummary := migrated.Accounts[0].Items[0]
			item, err := s.LoadItem(migrated, itemSummary.ID)
			require.NoError(t, err)
			require.Equal(t, "Private note", item.Title)
			vaults, err := s.Vaults()
			require.NoError(t, err)
			require.Len(t, vaults, 1)
			require.Equal(t, newID, vaults[0].ID)
			backups, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			foundBackup := false
			for _, entry := range backups {
				if len(entry.Name()) >= 4 && entry.Name()[len(entry.Name())-4:] == ".bck" {
					foundBackup = true
				}
			}
			require.True(t, foundBackup)
			entriesBeforeRetry, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			reloaded, err := s.LoadVault(newID, key)
			require.NoError(t, err)
			require.Equal(t, newID, reloaded.Accounts[0].ID)
			entriesAfterRetry, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entriesAfterRetry, len(entriesBeforeRetry))
		})
	}
}

// TestLoadVaultRejectsTruncatedLegacyVault checks corrupt vault data is not activated.
func TestLoadVaultRejectsTruncatedLegacyVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, _, _, vaultData := seedLegacyMigrationVault(t, s, true)
			truncated := vaultData[:len(vaultData)/2]
			require.NoError(t, os.WriteFile(legacyVaultPath(s, legacy.Name), truncated, 0600))

			_, err := s.LoadVault(legacy.Name, key)
			require.Error(t, err)
			require.Equal(t, truncated, readTestFile(t, legacyVaultPath(s, legacy.Name)))
			require.DirExists(t, legacyVaultRootPath(s, legacy.Name))
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

// TestLoadVaultRejectsUnsupportedCXFVersion checks unsupported documents are not activated.
func TestLoadVaultRejectsUnsupportedCXFVersion(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, _, _, _ := seedLegacyMigrationVault(t, s, true)
			vault, err := model.NewVault(key, legacy.Name)
			require.NoError(t, err)
			data, err := json.Marshal(vault)
			require.NoError(t, err)
			unsupported := strings.Replace(string(data), `"minor":0`, `"minor":1`, 1)
			require.NotEqual(t, string(data), unsupported)
			var encrypted bytes.Buffer
			writeEncrypted(t, key, &encrypted, []byte(unsupported))
			require.NoError(t, os.WriteFile(legacyVaultPath(s, legacy.Name), encrypted.Bytes(), 0600))

			_, err = s.LoadVault(legacy.Name, key)
			require.ErrorContains(t, err, "unsupported CXF version 1.1")
			require.DirExists(t, legacyVaultRootPath(s, legacy.Name))
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

// TestLoadVaultReportsTruncatedLegacyItem checks corrupt item data blocks activation.
func TestLoadVaultReportsTruncatedLegacyItem(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, note, _, _ := seedLegacyMigrationVault(t, s, true)
			path := legacyItemPath(s, legacy.Name, note.ID())
			itemData := readTestFile(t, path)
			truncated := itemData[:len(itemData)/2]
			require.NoError(t, os.WriteFile(path, truncated, 0600))

			_, err := s.LoadVault(legacy.Name, key)
			var report *MigrationReport
			require.ErrorAs(t, err, &report)
			require.Len(t, report.Failures, 1)
			require.Equal(t, note.ID(), report.Failures[0].LegacyID)
			require.Equal(t, []MigrationPhase{MigrationPhaseRead}, report.Failures[0].Phases)
			require.Equal(t, truncated, readTestFile(t, path))
			require.DirExists(t, legacyVaultRootPath(s, legacy.Name))
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

// TestLoadVaultKeyRejectsWrongPassword checks a bad password leaves the legacy vault untouched.
func TestLoadVaultKeyRejectsWrongPassword(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			keyData := &bytes.Buffer{}
			key, err := paw.MakeKey("correct password", keyData)
			require.NoError(t, err)
			legacy := paw.NewVault(key, "Personal")
			note := paw.NewNote()
			note.Name, note.Value = "Private note", "synthetic content"
			require.NoError(t, legacy.AddItem(note))
			legacyPath := legacyVaultRootPath(s, legacy.Name)
			require.NoError(t, os.MkdirAll(legacyPath, 0700))
			require.NoError(t, os.WriteFile(legacyKeyPath(s, legacy.Name), keyData.Bytes(), 0600))
			vaultJSON, err := json.Marshal(legacy)
			require.NoError(t, err)
			var encryptedVault bytes.Buffer
			writeEncrypted(t, key, &encryptedVault, vaultJSON)
			require.NoError(t, os.WriteFile(legacyVaultPath(s, legacy.Name), encryptedVault.Bytes(), 0600))
			noteJSON, err := json.Marshal(note)
			require.NoError(t, err)
			var encryptedNote bytes.Buffer
			writeEncrypted(t, key, &encryptedNote, noteJSON)
			require.NoError(t, os.WriteFile(legacyItemPath(s, legacy.Name, note.ID()), encryptedNote.Bytes(), 0600))
			keyFileBefore := readTestFile(t, legacyKeyPath(s, legacy.Name))
			vaultFileBefore := readTestFile(t, legacyVaultPath(s, legacy.Name))

			_, err = s.LoadVaultKey(legacy.Name, "wrong password")
			require.Error(t, err)
			require.Equal(t, keyFileBefore, readTestFile(t, legacyKeyPath(s, legacy.Name)))
			require.Equal(t, vaultFileBefore, readTestFile(t, legacyVaultPath(s, legacy.Name)))
			require.DirExists(t, legacyPath)
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 1)
		})
	}
}

// TestLoadVaultMigrationWriteFailureKeepsLegacyVault checks an unwritable storage root blocks staging.
func TestLoadVaultMigrationWriteFailureKeepsLegacyVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("directory permission bits do not prevent writes on Windows")
			}
			s := testCase.backend(t, t.TempDir())
			key, legacy, _, keyData, vaultData := seedLegacyMigrationVault(t, s, true)
			root := storageRootPath(s)
			require.NoError(t, os.Chmod(root, 0500))
			t.Cleanup(func() { _ = os.Chmod(root, 0700) })

			_, err := s.LoadVault(legacy.Name, key)
			require.Error(t, err)
			require.Contains(t, err.Error(), "could not create migration directory")
			require.DirExists(t, legacyVaultRootPath(s, legacy.Name))
			require.Equal(t, keyData, readTestFile(t, legacyKeyPath(s, legacy.Name)))
			require.Equal(t, vaultData, readTestFile(t, legacyVaultPath(s, legacy.Name)))
		})
	}
}

// TestLoadVaultMigrationBackupConflictKeepsBothVaults checks activation stops when its backup path exists.
func TestLoadVaultMigrationBackupConflictKeepsBothVaults(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, note, keyData, vaultData := seedLegacyMigrationVault(t, s, true)
			stageID, _ := createCompleteMigrationStage(t, s, legacy, note, keyData)
			root := storageRootPath(s)
			// Reserve nearby timestamps so the exact-second backup name is occupied.
			start := time.Now().UTC().Truncate(time.Second)
			for offset := 0; offset <= 60; offset++ {
				stamp := start.Add(time.Duration(offset) * time.Second).Format("060102_150405")
				name := legacy.Name + "." + stamp + vaultBackupSuffix
				require.NoError(t, os.Mkdir(filepath.Join(root, name), 0700))
			}

			_, err := s.LoadVault(legacy.Name, key)
			require.Error(t, err)
			require.Contains(t, err.Error(), "migration backup already exists")
			require.Contains(t, err.Error(), legacy.Name+".")
			require.DirExists(t, legacyVaultRootPath(s, legacy.Name))
			require.DirExists(t, vaultRootPath(s, stageID))
			require.Equal(t, keyData, readTestFile(t, legacyKeyPath(s, legacy.Name)))
			require.Equal(t, vaultData, readTestFile(t, legacyVaultPath(s, legacy.Name)))
		})
	}
}

// TestLoadVaultMigratesAllSupportedLegacyItems checks complete data survives migration on both backends.
func TestLoadVaultMigratesAllSupportedLegacyItems(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			legacyItems := supportedLegacyItems(t)
			key, legacy, _, _, _ := seedLegacyMigrationVaultWithItems(t, s, legacyItems...)

			vault, err := s.LoadVault(legacy.Name, key)
			require.NoError(t, err)
			require.Len(t, vault.Accounts[0].Items, len(legacyItems))

			for _, legacyItem := range legacyItems {
				expected, err := migrateTestItem(legacyItem)
				require.NoError(t, err)
				var summaryID string
				for _, summary := range vault.Accounts[0].Items {
					if summary.Title == legacyItem.GetMetadata().Name {
						summaryID = summary.ID
						break
					}
				}
				require.NotEmpty(t, summaryID, legacyItem.GetMetadata().Name)
				actual, err := s.LoadItem(vault, summaryID)
				require.NoError(t, err)
				expected.ID = actual.ID
				expectedJSON, err := json.Marshal(expected)
				require.NoError(t, err)
				actualJSON, err := json.Marshal(actual)
				require.NoError(t, err)
				require.JSONEq(t, string(expectedJSON), string(actualJSON))
				wantSummary, err := actual.Summary()
				require.NoError(t, err)
				require.Equal(t, *wantSummary, findItemSummary(t, vault, actual.ID))
			}
		})
	}
}

// TestLoadVaultMigrationFailureKeepsLegacyVault checks failed migration never publishes a partial CXF vault.
func TestLoadVaultMigrationFailureKeepsLegacyVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range []struct {
		name    string
		backend func(t *testing.T, root string) Storage
	}{
		{
			name: "OS",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewOSStorageRooted(root)
				require.NoError(t, err)
				return backend
			},
		},
		{
			name: "Fyne",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewFyneStorage(rootedFyneStorage{
					Storage: app.Storage(), root: fynestorage.NewFileURI(root),
				})
				require.NoError(t, err)
				return backend
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, note, keyData, vaultData := seedLegacyMigrationVault(t, s, false)
			legacyID := legacy.Name

			_, err := s.LoadVault(legacyID, key)
			var report *MigrationReport
			require.ErrorAs(t, err, &report)
			require.Equal(t, 1, report.TotalItems)
			require.Len(t, report.Failures, 1)
			failure := report.Failures[0]
			require.Equal(t, note.ID(), failure.LegacyID)
			require.Equal(t, "Private note", failure.Title)
			require.Equal(t, paw.NoteItemType, failure.ItemType)
			require.Equal(t, []MigrationPhase{MigrationPhaseRead}, failure.Phases)
			require.Contains(t, failure.Err.Error(), "could not create reader")

			require.DirExists(t, legacyVaultRootPath(s, legacyID))
			require.FileExists(t, legacyKeyPath(s, legacyID))
			actualKeyData, err := os.ReadFile(legacyKeyPath(s, legacyID))
			require.NoError(t, err)
			require.Equal(t, keyData, actualKeyData)
			actualVaultData, err := os.ReadFile(legacyVaultPath(s, legacyID))
			require.NoError(t, err)
			require.Equal(t, vaultData, actualVaultData)
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, legacyID, entries[0].Name())
			vaults, err := s.Vaults()
			require.NoError(t, err)
			require.Len(t, vaults, 1)
			require.Equal(t, legacyID, vaults[0].ID)
			require.Equal(t, legacyID, vaults[0].Name)
		})
	}
}

// TestLoadVaultMigrationConversionFailureKeepsLegacyVault checks unsupported password settings do not publish partial data.
func TestLoadVaultMigrationConversionFailureKeepsLegacyVault(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			secret := "SYNTHETIC_MIGRATION_SECRET"
			password := paw.NewPassword()
			password.Name = "Unsupported password"
			password.Value = secret
			password.Mode = paw.PasswordMode(4)
			_, legacy, _, keyData, vaultData := seedLegacyMigrationVaultWithItems(t, s, password)

			_, err := s.LoadVault(legacy.Name, legacy.Key())
			var report *MigrationReport
			require.ErrorAs(t, err, &report)
			require.Len(t, report.Failures, 1)
			require.Contains(t, err.Error(), "unsupported mode 4")
			require.NotContains(t, err.Error(), secret)
			require.DirExists(t, legacyVaultRootPath(s, legacy.Name))
			require.Equal(t, keyData, readTestFile(t, legacyKeyPath(s, legacy.Name)))
			require.Equal(t, vaultData, readTestFile(t, legacyVaultPath(s, legacy.Name)))
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 1)
			require.Equal(t, legacy.Name, entries[0].Name())
		})
	}
}

// TestLoadVaultReportsMetadataPublicationFailure checks both vault paths remain recoverable.
func TestLoadVaultReportsMetadataPublicationFailure(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			if runtime.GOOS == "windows" {
				t.Skip("directory permission bits do not prevent writes on Windows")
			}
			s := testCase.backend(t, t.TempDir())
			key, legacy, note, keyData, _ := seedLegacyMigrationVault(t, s, true)
			stageID, _ := createCompleteMigrationStage(t, s, legacy, note, keyData)
			stagePath := vaultRootPath(s, stageID)
			require.NoError(t, os.Chmod(stagePath, 0500))
			t.Cleanup(func() { _ = os.Chmod(stagePath, 0700) })

			_, err := s.LoadVault(legacy.Name, key)
			require.Error(t, err)
			require.Contains(t, err.Error(), "vault metadata could not be published")
			migrationErr := err
			require.Contains(t, migrationErr.Error(), stagePath)
			require.DirExists(t, stagePath)
			require.NoDirExists(t, legacyVaultRootPath(s, legacy.Name))
			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 2)
			var backup string
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), legacy.Name+".") && strings.HasSuffix(entry.Name(), vaultBackupSuffix) {
					backup = filepath.Join(storageRootPath(s), entry.Name())
				}
			}
			require.NotEmpty(t, backup)
			require.Contains(t, migrationErr.Error(), backup)
			require.DirExists(t, backup)
		})
	}
}

// TestVerifyMigratedItemRejectsChangedData checks verification fails on a changed item.
func TestVerifyMigratedItemRejectsChangedData(t *testing.T) {
	legacy := paw.NewNote()
	legacy.Name, legacy.Value = "Expected", "synthetic content"
	expected, err := migration.MigrateNote(legacy)
	require.NoError(t, err)
	actual := *expected
	actual.Title = "Changed"

	err = verifyMigratedItem(expected, &actual)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not match")
}

// TestLoadVaultReusesCompleteMigrationStage checks retries keep staged IDs in both backends.
func TestLoadVaultReusesCompleteMigrationStage(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range []struct {
		name    string
		backend func(t *testing.T, root string) Storage
	}{
		{
			name: "OS",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewOSStorageRooted(root)
				require.NoError(t, err)
				return backend
			},
		},
		{
			name: "Fyne",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewFyneStorage(rootedFyneStorage{
					Storage: app.Storage(), root: fynestorage.NewFileURI(root),
				})
				require.NoError(t, err)
				return backend
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, note, keyData, _ := seedLegacyMigrationVault(t, s, true)
			stageID, itemID := createCompleteMigrationStage(t, s, legacy, note, keyData)
			require.NoFileExists(t, vaultMetadataPath(s, stageID))

			migrated, err := s.LoadVault(legacy.Name, key)
			require.NoError(t, err)
			require.Equal(t, stageID, migrated.Accounts[0].ID)
			require.Len(t, migrated.Accounts[0].Items, 1)
			require.Equal(t, itemID, migrated.Accounts[0].Items[0].ID)
			_, err = s.LoadItem(migrated, itemID)
			require.NoError(t, err)

			entries, err := os.ReadDir(storageRootPath(s))
			require.NoError(t, err)
			require.Len(t, entries, 2)
			require.DirExists(t, vaultRootPath(s, stageID))
			require.NoDirExists(t, legacyVaultRootPath(s, legacy.Name))
			foundBackup := false
			for _, entry := range entries {
				if strings.HasPrefix(entry.Name(), legacy.Name+".") && strings.HasSuffix(entry.Name(), vaultBackupSuffix) {
					foundBackup = true
				}
			}
			require.True(t, foundBackup)
			vaults, err := s.Vaults()
			require.NoError(t, err)
			require.Len(t, vaults, 1)
			require.Equal(t, stageID, vaults[0].ID)
		})
	}
}

// TestLoadVaultRebuildsInvalidMigrationStage checks corrupted staged data is discarded and rebuilt.
func TestLoadVaultRebuildsInvalidMigrationStage(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)

	for _, testCase := range migrationBackends(t, app) {
		t.Run(testCase.name, func(t *testing.T) {
			s := testCase.backend(t, t.TempDir())
			key, legacy, note, keyData, _ := seedLegacyMigrationVault(t, s, true)
			stageID, itemID := createCompleteMigrationStage(t, s, legacy, note, keyData)
			require.NoError(t, os.WriteFile(itemPath(s, stageID, itemID), []byte("corrupt stage item"), 0600))

			migrated, err := s.LoadVault(legacy.Name, key)
			require.NoError(t, err)
			require.NotEqual(t, stageID, migrated.Accounts[0].ID)
			require.NoDirExists(t, vaultRootPath(s, stageID))
			require.NoDirExists(t, legacyVaultRootPath(s, legacy.Name))
			require.Len(t, migrated.Accounts[0].Items, 1)
			item, err := s.LoadItem(migrated, migrated.Accounts[0].Items[0].ID)
			require.NoError(t, err)
			require.Equal(t, note.Name, item.Title)
		})
	}
}

// createCompleteMigrationStage writes a converted item without publishing vault metadata.
func createCompleteMigrationStage(t *testing.T, s Storage, legacy *paw.Vault, note *paw.Note, keyData []byte) (string, string) {
	t.Helper()
	stage, err := migrationVault(legacy)
	require.NoError(t, err)
	stageID := stage.Accounts[0].ID
	require.NoError(t, fynestorage.CreateListable(fynestorage.NewFileURI(vaultRootPath(s, stageID))))
	require.NoError(t, os.WriteFile(keyPath(s, stageID), keyData, 0600))
	item, err := convertLegacyItem(note)
	require.NoError(t, err)
	require.NoError(t, s.StoreItem(stage, item))
	return stageID, item.ID
}

// seedLegacyMigrationVault writes a legacy vault with an optional item file.
func seedLegacyMigrationVault(t *testing.T, s Storage, saveItem bool) (*paw.Key, *paw.Vault, *paw.Note, []byte, []byte) {
	t.Helper()
	note := paw.NewNote()
	note.Name, note.Value = "Private note", "keep this content"
	key, legacy, _, keyData, vaultData := seedLegacyMigrationVaultWithItems(t, s, note)
	if !saveItem {
		itemFile := legacyItemPath(s, legacy.Name, note.ID())
		require.NoError(t, os.Remove(itemFile))
	}
	return key, legacy, note, keyData, vaultData
}

// seedLegacyMigrationVaultWithItems writes an encrypted Paw vault and its item files.
func seedLegacyMigrationVaultWithItems(t *testing.T, s Storage, items ...paw.Item) (*paw.Key, *paw.Vault, []paw.Item, []byte, []byte) {
	t.Helper()
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	legacy := paw.NewVault(key, "Personal")
	for _, item := range items {
		require.NoError(t, legacy.AddItem(item))
	}
	legacyID := legacy.Name
	require.NoError(t, os.MkdirAll(legacyVaultRootPath(s, legacyID), 0700))
	keyData := []byte("unchanged encrypted key file")
	require.NoError(t, os.WriteFile(legacyKeyPath(s, legacyID), keyData, 0600))
	vaultJSON, err := json.Marshal(legacy)
	require.NoError(t, err)
	var encryptedVault bytes.Buffer
	writeEncrypted(t, key, &encryptedVault, vaultJSON)
	vaultData := append([]byte(nil), encryptedVault.Bytes()...)
	require.NoError(t, os.WriteFile(legacyVaultPath(s, legacyID), vaultData, 0600))
	for _, item := range items {
		itemJSON, err := json.Marshal(item)
		require.NoError(t, err)
		var encryptedItem bytes.Buffer
		writeEncrypted(t, key, &encryptedItem, itemJSON)
		require.NoError(t, os.WriteFile(legacyItemPath(s, legacyID, item.ID()), encryptedItem.Bytes(), 0600))
	}
	return key, legacy, items, keyData, vaultData
}

// migrationBackends returns the storage implementations exercised by migration end-to-end tests.
func migrationBackends(t *testing.T, app fyne.App) []struct {
	name    string
	backend func(t *testing.T, root string) Storage
} {
	t.Helper()
	return []struct {
		name    string
		backend func(t *testing.T, root string) Storage
	}{
		{
			name: "OS",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewOSStorageRooted(root)
				require.NoError(t, err)
				return backend
			},
		},
		{
			name: "Fyne",
			backend: func(t *testing.T, root string) Storage {
				backend, err := NewFyneStorage(rootedFyneStorage{
					Storage: app.Storage(), root: fynestorage.NewFileURI(root),
				})
				require.NoError(t, err)
				return backend
			},
		},
	}
}

// supportedLegacyItems returns synthetic Paw items for every supported legacy type.
func supportedLegacyItems(t *testing.T) []paw.Item {
	t.Helper()
	note := paw.NewNote()
	note.Name, note.Value = "Legacy note", "synthetic note content"
	note.Metadata.Subtitle = "note subtitle"
	note.Metadata.Created = time.Unix(100, 0).UTC()
	note.Metadata.Modified = time.Unix(200, 0).UTC()

	password := paw.NewPassword()
	password.Name, password.Value = "Legacy password", "synthetic saved password"
	password.Mode, password.Length, password.Format = paw.RandomPassword, 24, paw.LowercaseFormat|paw.DigitsFormat
	password.Metadata.Created = time.Unix(300, 0).UTC()
	password.Metadata.Modified = time.Unix(400, 0).UTC()

	login := paw.NewLogin()
	login.Name, login.Username, login.Password.Value = "Legacy login", "user@example.com", "synthetic login password"
	login.Password.Mode, login.Password.Length = paw.PassphrasePassword, 5
	login.TOTP = &paw.TOTP{Secret: "JBSWY3DPEHPK3PXP", Digits: 6, Hash: paw.SHA256, Interval: 30}
	login.Note.Value = "synthetic login note"
	require.NoError(t, login.URL.Set("https://example.com/login"))
	login.Metadata.Created = time.Unix(500, 0).UTC()
	login.Metadata.Modified = time.Unix(600, 0).UTC()

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(privateKey, "synthetic test key")
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(privateKey)
	require.NoError(t, err)
	sshKey := paw.NewSSHKey()
	sshKey.Name, sshKey.Comment = "Legacy SSH key", "synthetic@example.com"
	sshKey.PrivateKey = string(pem.EncodeToMemory(block))
	sshKey.PublicKey = string(ssh.MarshalAuthorizedKey(signer.PublicKey()))
	sshKey.Fingerprint = ssh.FingerprintSHA256(signer.PublicKey())
	sshKey.Passphrase = &paw.Password{Value: "synthetic passphrase", Mode: paw.CustomPassword}
	sshKey.Note.Value = "synthetic SSH note"
	sshKey.AddToAgent = true
	sshKey.Metadata.Created = time.Unix(700, 0).UTC()
	sshKey.Metadata.Modified = time.Unix(800, 0).UTC()

	return []paw.Item{note, password, login, sshKey}
}

// migrateTestItem converts one fixture with the same converter used by storage migration.
func migrateTestItem(item paw.Item) (*model.Item, error) {
	switch value := item.(type) {
	case *paw.Note:
		return migration.MigrateNote(value)
	case *paw.Password:
		return migration.MigratePassword(value)
	case *paw.Login:
		return migration.MigrateLogin(value)
	case *paw.SSHKey:
		return migration.MigrateSSHKey(value)
	default:
		return nil, fmt.Errorf("unsupported test item type")
	}
}

// findItemSummary returns the vault summary matching an item ID.
func findItemSummary(t *testing.T, vault *model.Vault, itemID string) cxf.Item {
	t.Helper()
	for _, summary := range vault.Accounts[0].Items {
		if summary.ID == itemID {
			return summary
		}
	}
	t.Fatalf("item summary %q was not found", itemID)
	return cxf.Item{}
}

// readTestFile reads a migration fixture file and fails the test on errors.
func readTestFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}
