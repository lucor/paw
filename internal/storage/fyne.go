// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	fynestorage "fyne.io/fyne/v2/storage"

	"lucor.dev/paw/internal/model"
	"lucor.dev/paw/internal/paw"
)

var _ Storage = (*FyneStorage)(nil)

// FyneStorage stores encrypted vault data through Fyne's storage API.
type FyneStorage struct {
	// Storage provides the application's data directory.
	fyne.Storage
}

// NewFyneStorage returns an Fyne Storage implementation
func NewFyneStorage(s fyne.Storage) (Storage, error) {
	fs := &FyneStorage{Storage: s}
	err := fs.mkdirIfNotExists(storageRootPath(fs))
	if err != nil {
		return nil, fmt.Errorf("could not create storage dir: %w", err)
	}
	return fs, nil
}

// Root returns the Paw data directory.
func (s *FyneStorage) Root() string {
	return s.Storage.RootURI().Path()
}

// DeleteVault removes a vault directory by its canonical ID through Fyne storage.
func (s *FyneStorage) DeleteVault(vaultID string) error {
	if !isVaultID(vaultID) {
		return fmt.Errorf("invalid vault ID")
	}
	if err := fynestorage.DeleteAll(fynestorage.NewFileURI(vaultRootPath(s, vaultID))); err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultKey decrypts the vault key using its canonical ID and password.
func (s *FyneStorage) LoadVaultKey(vaultID string, password string) (*paw.Key, error) {
	keyFile := keyPath(s, vaultID)
	if !isVaultID(vaultID) {
		if !isLegacyVaultName(vaultID) {
			return nil, fmt.Errorf("invalid vault ID")
		}
		keyFile = legacyKeyPath(s, vaultID)
	}
	r, err := fynestorage.Reader(fynestorage.NewFileURI(keyFile))
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return paw.LoadKey(password, r)
}

// storeVaultInfo writes the public name used to list a CXF vault.
func (s *FyneStorage) storeVaultInfo(info VaultInfo) error {
	if !isVaultID(info.ID) {
		return fmt.Errorf("invalid CXF vault ID")
	}
	if err := s.mkdirIfNotExists(vaultRootPath(s, info.ID)); err != nil {
		return fmt.Errorf("could not create vault directory: %w", err)
	}
	w, err := s.createFile(vaultMetadataPath(s, info.ID))
	if err != nil {
		return fmt.Errorf("could not create vault metadata: %w", err)
	}
	writeErr := writeVaultMetadata(w, info)
	closeErr := w.Close()
	return errors.Join(writeErr, closeErr)
}

// Vaults returns vault IDs and display names, including unmigrated legacy vaults.
func (s *FyneStorage) Vaults() ([]VaultInfo, error) {
	root := fynestorage.NewFileURI(storageRootPath(s))
	vaults := []VaultInfo{}
	dirEntries, err := fynestorage.List(root)
	if err != nil {
		return nil, err
	}
	for _, dirEntry := range dirEntries {
		if strings.HasSuffix(dirEntry.Name(), vaultBackupSuffix) {
			continue
		}
		if ok, err := fynestorage.CanList(dirEntry); !ok {
			if err != nil {
				fyne.LogError("could not list dir entry", err)
			}
			continue
		}
		if !isFilesystemVaultID(dirEntry.Name()) {
			vaults = append(vaults, VaultInfo{ID: dirEntry.Name(), Name: dirEntry.Name(), NeedsMigration: true})
			continue
		}
		metadataPath := filepath.Join(storageRootPath(s), dirEntry.Name(), vaultMetadataFileName)
		metadataURI := fynestorage.NewFileURI(metadataPath)
		reader, err := fynestorage.Reader(metadataURI)
		if err != nil {
			if !s.isExist(metadataPath) {
				continue
			}
			return nil, fmt.Errorf("could not read vault metadata: %w", err)
		}
		data, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, fmt.Errorf("could not read vault metadata: %w", err)
		}
		info, err := vaultInfoFromFilesystemName(dirEntry.Name(), data)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata for vault %q: %w", dirEntry.Name(), err)
		}
		vaults = append(vaults, info)
	}
	return vaults, nil
}

// LoadAppState load the configuration from the underlying storage
func (s *FyneStorage) LoadAppState() (*paw.AppState, error) {
	defaultAppState := &paw.AppState{
		Modified:    time.Now().UTC(),
		Preferences: paw.NewDefaultPreferences(),
	}
	appStateFile := appStateFilePath(s)
	r, err := fynestorage.Reader(fynestorage.NewFileURI(appStateFile))
	if err != nil {
		return defaultAppState, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	appState := &paw.AppState{}
	err = json.NewDecoder(r).Decode(appState)
	if err != nil {
		return defaultAppState, err
	}
	return appState, nil
}

// StoreAppState store the configuration into the underlying storage
func (s *FyneStorage) StoreAppState(appState *paw.AppState) error {
	appStateFile := appStateFilePath(s)
	w, err := s.createFile(appStateFile)
	if err != nil {
		return err
	}
	err = json.NewEncoder(w).Encode(appState)
	return errors.Join(err, w.Close())
}

// SocketAgentPath return the socket agent path
func (s *FyneStorage) SocketAgentPath() string { return socketAgentPath(s) }

// LockFilePath return the lock file path
func (s *FyneStorage) LockFilePath() string { return lockFilePath(s) }

// LogFilePath return the log file path
func (s *FyneStorage) LogFilePath() string { return logFilePath(s) }

// isExist reports whether a path exists.
func (s *FyneStorage) isExist(path string) bool {
	ok, _ := fynestorage.Exists(fynestorage.NewFileURI(path))
	return ok
}

// mkdirIfNotExists creates a directory when it is missing.
func (s *FyneStorage) mkdirIfNotExists(path string) error {
	if s.isExist(path) {
		return nil
	}
	return fynestorage.CreateListable(fynestorage.NewFileURI(path))
}

// createFile opens a file, replacing its current contents.
func (s *FyneStorage) createFile(name string) (fyne.URIWriteCloser, error) {
	return fynestorage.Writer(fynestorage.NewFileURI(name))
}

// CreateVault saves a new CXF vault before publishing its catalog entry.
func (s *FyneStorage) CreateVault(name, password string) (*model.Vault, error) {
	vault, keyData, err := prepareVault(name, password)
	if err != nil {
		return nil, err
	}
	id := vault.Accounts[0].ID
	if err := fynestorage.CreateListable(fynestorage.NewFileURI(vaultRootPath(s, id))); err != nil {
		return nil, fmt.Errorf("could not create vault directory: %w", err)
	}
	writer, err := s.createFile(keyPath(s, id))
	if err != nil {
		return nil, fmt.Errorf("could not create vault key file: %w", err)
	}
	_, writeErr := io.Copy(writer, bytes.NewReader(keyData))
	if err := errors.Join(writeErr, writer.Close()); err != nil {
		return nil, fmt.Errorf("could not store vault key: %w", err)
	}
	if err := s.storeEncrypted(vaultPath(s, id), vault.Key(), vault); err != nil {
		return nil, fmt.Errorf("could not store vault: %w", err)
	}
	if err := s.publishVault(VaultInfo{ID: id, Name: name}); err != nil {
		return nil, err
	}
	return vault, nil
}

// LoadVault reads a CXF vault or migrates a legacy vault before returning it.
func (s *FyneStorage) LoadVault(vaultID string, key *paw.Key) (*model.Vault, error) {
	if !isVaultID(vaultID) {
		if !isLegacyVaultName(vaultID) {
			return nil, fmt.Errorf("invalid vault ID")
		}
		data, err := s.readLegacyVaultData(vaultID, key)
		if err != nil {
			return nil, err
		}
		if isLegacyVault(data) {
			legacy, err := paw.DecodeVault(data, key)
			if err != nil {
				return nil, err
			}
			return s.migrateVault(legacy)
		}
		vault, err := model.DecodeVault(data, key)
		if err != nil {
			return nil, fmt.Errorf("could not decode CXF vault: %w", err)
		}
		if vault.Accounts[0].ID != vaultID {
			return nil, fmt.Errorf("vault ID does not match requested ID")
		}
		return vault, nil
	}
	r, err := fynestorage.Reader(fynestorage.NewFileURI(vaultPath(s, vaultID)))
	if err != nil {
		return nil, fmt.Errorf("could not open vault: %w", err)
	}
	defer r.Close()
	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt vault: %w", err)
	}
	if isLegacyVault(data) {
		legacy, err := paw.DecodeVault(data, key)
		if err != nil {
			return nil, err
		}
		return s.migrateVault(legacy)
	}
	vault, err := model.DecodeVault(data, key)
	if err != nil {
		return nil, fmt.Errorf("could not decode CXF vault: %w", err)
	}
	if vault.Accounts[0].ID != vaultID {
		return nil, fmt.Errorf("vault ID does not match requested ID")
	}
	return vault, nil
}

// migrateVault converts a legacy vault into a complete CXF vault before activation.
func (s *FyneStorage) migrateVault(legacy *paw.Vault) (*model.Vault, error) {
	keyReader, err := fynestorage.Reader(fynestorage.NewFileURI(legacyKeyPath(s, legacy.Name)))
	if err != nil {
		return nil, fmt.Errorf("could not read legacy vault key: %w", err)
	}
	keyData, err := io.ReadAll(keyReader)
	closeErr := keyReader.Close()
	if err := errors.Join(err, closeErr); err != nil {
		return nil, fmt.Errorf("could not read legacy vault key: %w", err)
	}
	vault, err := s.findCompleteMigrationStage(legacy, keyData)
	if err != nil {
		return nil, err
	}
	createdStage := vault == nil
	if createdStage {
		vault, err = migrationVault(legacy)
		if err != nil {
			return nil, fmt.Errorf("could not prepare vault migration: %w", err)
		}
	}
	id := vault.Accounts[0].ID
	stageURI := fynestorage.NewFileURI(vaultRootPath(s, id))
	if createdStage {
		if err := fynestorage.CreateListable(stageURI); err != nil {
			return nil, fmt.Errorf("could not create migration directory: %w", err)
		}
	}
	stageComplete := !createdStage
	defer func() {
		if !stageComplete {
			_ = fynestorage.DeleteAll(stageURI)
		}
	}()

	verified := vault
	if createdStage {
		keyWriter, err := s.createFile(keyPath(s, id))
		if err != nil {
			return nil, fmt.Errorf("could not create migrated vault key: %w", err)
		}
		_, copyErr := io.Copy(keyWriter, bytes.NewReader(keyData))
		if err := errors.Join(copyErr, keyWriter.Close()); err != nil {
			return nil, fmt.Errorf("could not copy legacy vault key: %w", err)
		}

		report := &MigrationReport{TotalItems: legacy.Size()}
		converted := make([]*model.Item, 0, report.TotalItems)
		for _, ref := range legacyItems(legacy) {
			if ref.metadata == nil {
				report.add(MigrationItemFailure{LegacyID: ref.id, Err: fmt.Errorf("legacy item metadata is missing")}, MigrationPhaseRead)
				continue
			}
			oldItem, err := s.loadLegacyItem(legacy, ref.metadata)
			if err != nil {
				report.add(MigrationItemFailure{LegacyID: ref.id, Title: ref.metadata.Name, ItemType: ref.metadata.Type, Err: err}, MigrationPhaseRead)
				continue
			}
			item, err := convertLegacyItem(oldItem)
			if err != nil {
				report.add(MigrationItemFailure{LegacyID: ref.id, Title: ref.metadata.Name, ItemType: ref.metadata.Type, Err: err}, MigrationPhaseConvert)
				continue
			}
			updated, err := prepareItemVault(vault, item)
			if err != nil {
				report.add(MigrationItemFailure{LegacyID: ref.id, Title: ref.metadata.Name, ItemType: ref.metadata.Type, Err: err}, MigrationPhaseValidate)
				continue
			}
			if err := s.storeEncrypted(itemPath(s, id, item.ID), vault.Key(), item); err != nil {
				return nil, fmt.Errorf("could not write migrated item %q: %w", ref.metadata.Name, err)
			}
			vault = updated
			converted = append(converted, item)
		}
		if len(report.Failures) > 0 {
			return nil, report
		}
		vault.Timestamp = uint64(time.Now().Unix())
		if err := s.storeEncrypted(vaultPath(s, id), vault.Key(), vault); err != nil {
			return nil, fmt.Errorf("could not write migrated vault: %w", err)
		}
		verified, err = s.LoadVault(id, vault.Key())
		if err != nil {
			return nil, fmt.Errorf("could not verify migrated vault: %w", err)
		}
		for _, item := range converted {
			actual, err := s.LoadItem(verified, item.ID)
			if err != nil {
				return nil, fmt.Errorf("could not verify migrated item %q: %w", item.Title, err)
			}
			if err := verifyMigratedItem(item, actual); err != nil {
				return nil, fmt.Errorf("could not verify migrated item %q: %w", item.Title, err)
			}
		}
	}

	// Keep a complete stage if activation fails so the retry can reuse its IDs.
	stageComplete = true
	backup := filepath.Join(storageRootPath(s), legacy.Name+"."+time.Now().UTC().Format("060102_150405")+vaultBackupSuffix)
	backupURI := fynestorage.NewFileURI(backup)
	exists, err := fynestorage.Exists(backupURI)
	if err != nil {
		return nil, fmt.Errorf("could not inspect migration backup path: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("migration backup already exists: %s", backup)
	}
	if err := fynestorage.Move(fynestorage.NewFileURI(legacyVaultRootPath(s, legacy.Name)), backupURI); err != nil {
		return nil, fmt.Errorf("could not preserve legacy vault at %s: %w", backup, err)
	}
	if err := s.publishVault(VaultInfo{ID: id, Name: legacy.Name}); err != nil {
		return nil, fmt.Errorf("migration is complete but vault metadata could not be published; backup: %s, new vault: %s: %w", backup, vaultRootPath(s, id), err)
	}
	return verified, nil
}

// findCompleteMigrationStage reuses a valid unpublished stage for this vault.
func (s *FyneStorage) findCompleteMigrationStage(legacy *paw.Vault, keyData []byte) (*model.Vault, error) {
	entries, err := fynestorage.List(fynestorage.NewFileURI(storageRootPath(s)))
	if err != nil {
		return nil, err
	}
	var complete *model.Vault
	for _, entry := range entries {
		id, ok := idFromFilesystemName(entry.Name())
		if !ok {
			continue
		}
		stageDir := filepath.Join(storageRootPath(s), entry.Name())
		metadataExists, err := fynestorage.Exists(fynestorage.NewFileURI(vaultMetadataPath(s, id)))
		if err != nil {
			return nil, fmt.Errorf("could not inspect migration stage metadata: %w", err)
		}
		if metadataExists {
			continue
		}
		keyReader, err := fynestorage.Reader(fynestorage.NewFileURI(keyPath(s, id)))
		if err != nil {
			continue
		}
		stageKey, readErr := io.ReadAll(keyReader)
		closeErr := keyReader.Close()
		if err := errors.Join(readErr, closeErr); err != nil || !bytes.Equal(stageKey, keyData) {
			continue
		}
		vaultReader, err := fynestorage.Reader(fynestorage.NewFileURI(vaultPath(s, id)))
		if err != nil {
			if removeErr := fynestorage.DeleteAll(fynestorage.NewFileURI(stageDir)); removeErr != nil {
				return nil, fmt.Errorf("could not remove incomplete migration stage: %w", removeErr)
			}
			continue
		}
		data, readErr := readDecrypted(legacy.Key(), vaultReader)
		closeErr = vaultReader.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			if removeErr := fynestorage.DeleteAll(fynestorage.NewFileURI(stageDir)); removeErr != nil {
				return nil, fmt.Errorf("could not remove incomplete migration stage: %w", removeErr)
			}
			continue
		}
		stage, err := model.DecodeVault(data, legacy.Key())
		if err != nil || stage.Metadata.VaultName != legacy.Name || len(stage.Accounts[0].Items) != legacy.Size() {
			if removeErr := fynestorage.DeleteAll(fynestorage.NewFileURI(stageDir)); removeErr != nil {
				return nil, fmt.Errorf("could not remove incomplete migration stage: %w", removeErr)
			}
			continue
		}
		valid := true
		for _, summary := range stage.Accounts[0].Items {
			if _, err := s.LoadItem(stage, summary.ID); err != nil {
				valid = false
				break
			}
		}
		if !valid {
			if removeErr := fynestorage.DeleteAll(fynestorage.NewFileURI(stageDir)); removeErr != nil {
				return nil, fmt.Errorf("could not remove incomplete migration stage: %w", removeErr)
			}
			continue
		}
		if complete != nil {
			return nil, fmt.Errorf("multiple complete migration stages match vault %q", legacy.Name)
		}
		complete = stage
	}
	return complete, nil
}

// LoadItem reads and validates an encrypted CXF item by ID.
func (s *FyneStorage) LoadItem(vault *model.Vault, itemID string) (*model.Item, error) {
	if !vaultHasItem(vault, itemID) {
		return nil, fmt.Errorf("item is not in the vault")
	}
	r, err := fynestorage.Reader(fynestorage.NewFileURI(itemPath(s, vault.Accounts[0].ID, itemID)))
	if err != nil {
		return nil, fmt.Errorf("could not open item: %w", err)
	}
	defer r.Close()
	data, err := readDecrypted(vault.Key(), r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt item: %w", err)
	}
	item, err := decodeModelItem(data)
	if err != nil {
		return nil, err
	}
	if item.ID != itemID {
		return nil, fmt.Errorf("item ID does not match requested ID")
	}
	return item, nil
}

// StoreItem saves a CXF item and updates its vault summary.
func (s *FyneStorage) StoreItem(vault *model.Vault, item *model.Item) error {
	return s.storeItem(vault, item)
}

// DeleteItem removes a CXF item and its vault summary.
func (s *FyneStorage) DeleteItem(vault *model.Vault, itemID string) error {
	return s.deleteItem(vault, itemID)
}

// RenameVault updates the encrypted vault name and its catalog entry.
func (s *FyneStorage) RenameVault(vault *model.Vault, name string) error {
	updated := *vault
	updated.Metadata.VaultName = name
	updated.Accounts = cloneVaultAccounts(vault.Accounts)
	updated.Timestamp = uint64(time.Now().Unix())
	if err := updated.Validate(); err != nil {
		return fmt.Errorf("cannot rename vault: %w", err)
	}
	id := updated.Accounts[0].ID
	if err := s.storeEncrypted(vaultPath(s, id), vault.Key(), &updated); err != nil {
		return fmt.Errorf("could not store renamed vault: %w", err)
	}
	if err := s.storeVaultInfo(VaultInfo{ID: id, Name: name}); err != nil {
		return fmt.Errorf("could not update vault catalog: %w", err)
	}
	*vault = updated
	return nil
}

// storeItem saves the full item before saving its summary and advancing the vault.
func (s *FyneStorage) storeItem(vault *model.Vault, item *model.Item) error {
	updated, err := prepareItemVault(vault, item)
	if err != nil {
		return err
	}
	id := updated.Accounts[0].ID
	if err := s.storeEncrypted(itemPath(s, id, item.ID), vault.Key(), item); err != nil {
		return fmt.Errorf("could not store item: %w", err)
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncrypted(vaultPath(s, id), vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was saved: %w", err)
	}
	*vault = *updated
	return nil
}

// deleteItem removes the encrypted item before saving the updated vault summary.
func (s *FyneStorage) deleteItem(vault *model.Vault, itemID string) error {
	updated, err := prepareItemDeleteVault(vault, itemID)
	if err != nil {
		return err
	}
	id := updated.Accounts[0].ID
	if err := fynestorage.Delete(fynestorage.NewFileURI(itemPath(s, id, itemID))); err != nil {
		return fmt.Errorf("could not delete item file: %w", err)
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncrypted(vaultPath(s, id), vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was deleted: %w", err)
	}
	*vault = *updated
	return nil
}

// storeEncrypted opens a Fyne file and checks encryption, write, and close errors.
func (s *FyneStorage) storeEncrypted(path string, key *paw.Key, value any) error {
	writer, err := s.createFile(path)
	if err != nil {
		return err
	}
	return encryptFile(key, writer, value)
}

// publishVault removes an incomplete catalog entry when publication fails.
func (s *FyneStorage) publishVault(info VaultInfo) error {
	// Check before cleanup too: an invalid ID must never select a path to remove.
	if !isVaultID(info.ID) {
		return fmt.Errorf("could not publish vault metadata: invalid CXF vault ID")
	}
	if err := s.storeVaultInfo(info); err != nil {
		publishErr := fmt.Errorf("could not publish vault metadata: %w", err)
		uri := fynestorage.NewFileURI(vaultMetadataPath(s, info.ID))
		exists, checkErr := fynestorage.Exists(uri)
		if checkErr != nil {
			return errors.Join(publishErr, fmt.Errorf("could not inspect incomplete vault metadata: %w", checkErr))
		}
		if exists {
			if removeErr := fynestorage.Delete(uri); removeErr != nil {
				return errors.Join(publishErr, fmt.Errorf("could not remove incomplete vault metadata: %w", removeErr))
			}
		}
		return publishErr
	}
	return nil
}
