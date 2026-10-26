// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

var _ Storage = (*OSStorage)(nil)

// OSStorage stores encrypted vault data in the local filesystem.
type OSStorage struct {
	root string
}

// NewOSStorage uses PAW_HOME or the user's home directory as its parent directory.
func NewOSStorage() (Storage, error) {
	var root string
	var err error
	root = os.Getenv(paw.ENV_HOME)
	if root == "" {
		root, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not get the default root directory to use for user-specific configuration data: %w", err)
		}
	}
	return newOSStorageRooted(root, true)
}

// NewOSStorageRooted returns an OS storage implementation rooted at root.
func NewOSStorageRooted(root string) (Storage, error) {
	return newOSStorageRooted(root, false)
}

// newOSStorageRooted creates storage and optionally imports the old Fyne location.
func newOSStorageRooted(root string, migrateDeprecated bool) (Storage, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("storage root must be an absolute path, got %s", root)
	}
	// Fyne does not allow to customize the root for a storage
	// so we'll use the same
	storageRoot := filepath.Join(root, ".paw")
	s := &OSStorage{root: storageRoot}
	if migrateDeprecated {
		migrated, err := s.migrateDeprecatedRootStorage()
		if migrated {
			if err != nil {
				return nil, fmt.Errorf("found deprecated storage but was unable to move to new location: %w", err)
			}
			return s, nil
		}
	}
	err := s.mkdirIfNotExists(storageRootPath(s))
	return s, err
}

// Root returns the Paw data directory.
func (s *OSStorage) Root() string {
	return s.root
}

// DeleteVault removes a vault directory by its canonical ID.
func (s *OSStorage) DeleteVault(vaultID string) error {
	if !isVaultID(vaultID) {
		return fmt.Errorf("invalid vault ID")
	}
	if err := os.RemoveAll(vaultRootPath(s, vaultID)); err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultKey decrypts the vault key using its canonical ID and password.
func (s *OSStorage) LoadVaultKey(vaultID, password string) (*paw.Key, error) {
	keyFile := keyPath(s, vaultID)
	if !isVaultID(vaultID) {
		if !isLegacyVaultName(vaultID) {
			return nil, fmt.Errorf("invalid vault ID")
		}
		keyFile = legacyKeyPath(s, vaultID)
	}
	r, err := os.Open(keyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return paw.LoadKey(password, r)
}

// storeVaultInfo writes the public name used to list a CXF vault.
func (s *OSStorage) storeVaultInfo(info VaultInfo) error {
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
func (s *OSStorage) Vaults() ([]VaultInfo, error) {
	entries, err := os.ReadDir(storageRootPath(s))
	if err != nil {
		return nil, err
	}
	vaults := []VaultInfo{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasSuffix(entry.Name(), vaultBackupSuffix) {
			continue
		}
		if !isFilesystemVaultID(entry.Name()) {
			vaults = append(vaults, VaultInfo{ID: entry.Name(), Name: entry.Name(), NeedsMigration: true})
			continue
		}
		metadataPath := filepath.Join(storageRootPath(s), entry.Name(), vaultMetadataFileName)
		data, err := os.ReadFile(metadataPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not read vault metadata: %w", err)
		}
		info, err := vaultInfoFromFilesystemName(entry.Name(), data)
		if err != nil {
			return nil, fmt.Errorf("invalid metadata for vault %q: %w", entry.Name(), err)
		}
		vaults = append(vaults, info)
	}
	return vaults, nil
}

// LoadAppState load the configuration from the underlying storage
func (s *OSStorage) LoadAppState() (*paw.AppState, error) {
	defaultAppState := &paw.AppState{
		Modified:    time.Now().UTC(),
		Preferences: paw.NewDefaultPreferences(),
	}
	appStateFile := appStateFilePath(s)
	r, err := os.Open(appStateFile)
	if os.IsNotExist(err) {
		return defaultAppState, nil
	}
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
func (s *OSStorage) StoreAppState(appState *paw.AppState) error {
	appStateFile := appStateFilePath(s)
	w, err := s.createFile(appStateFile)
	if err != nil {
		return err
	}
	err = json.NewEncoder(w).Encode(appState)
	return errors.Join(err, w.Close())
}

// SocketAgentPath returns the socket agent path.
func (s *OSStorage) SocketAgentPath() string { return socketAgentPath(s) }

// LockFilePath returns the lock file path.
func (s *OSStorage) LockFilePath() string { return lockFilePath(s) }

// LogFilePath returns the log file path.
func (s *OSStorage) LogFilePath() string { return logFilePath(s) }

// isExist reports whether a path exists.
func (s *OSStorage) isExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// mkdirIfNotExists creates a private directory when it is missing.
func (s *OSStorage) mkdirIfNotExists(path string) error {
	if s.isExist(path) {
		return nil
	}
	return os.MkdirAll(path, 0700)
}

// createFile opens a private file, replacing its current contents.
func (s *OSStorage) createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
}

// migrateDeprecatedRootStorage moves vaults from the old Fyne data directory.
func (s *OSStorage) migrateDeprecatedRootStorage() (bool, error) {
	oldRoot, err := os.UserConfigDir()
	if err != nil {
		return false, nil
	}
	src := filepath.Join(oldRoot, "fyne", paw.ID, "vaults")
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return false, nil
	}
	return true, os.Rename(src, storageRootPath(s))
}

// CreateVault saves a new CXF vault before publishing its catalog entry.
func (s *OSStorage) CreateVault(name, password string) (*model.Vault, error) {
	vault, keyData, err := prepareVault(name, password)
	if err != nil {
		return nil, err
	}
	id := vault.Accounts[0].ID
	if err := os.Mkdir(vaultRootPath(s, id), 0700); err != nil {
		return nil, fmt.Errorf("could not create vault directory: %w", err)
	}
	if err := os.WriteFile(keyPath(s, id), keyData, 0600); err != nil {
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
func (s *OSStorage) LoadVault(vaultID string, key *paw.Key) (*model.Vault, error) {
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
	data, err := s.readVaultData(vaultID, key)
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

// migrateVault converts a legacy vault into a complete CXF vault before activation.
func (s *OSStorage) migrateVault(legacy *paw.Vault) (*model.Vault, error) {
	keyData, err := os.ReadFile(legacyKeyPath(s, legacy.Name))
	if err != nil {
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
	if createdStage {
		if err := os.Mkdir(vaultRootPath(s, id), 0700); err != nil {
			return nil, fmt.Errorf("could not create migration directory: %w", err)
		}
	}
	stageComplete := !createdStage
	defer func() {
		if !stageComplete {
			_ = os.RemoveAll(vaultRootPath(s, id))
		}
	}()

	verified := vault
	if createdStage {
		if err := os.WriteFile(keyPath(s, id), keyData, 0600); err != nil {
			return nil, fmt.Errorf("could not copy legacy vault key: %w", err)
		}
		converted := make([]*model.Item, 0, legacy.Size())
		report := &MigrationReport{TotalItems: legacy.Size()}
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
	if _, err := os.Lstat(backup); err == nil {
		return nil, fmt.Errorf("migration backup already exists: %s", backup)
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("could not inspect migration backup path: %w", err)
	}
	if err := os.Rename(legacyVaultRootPath(s, legacy.Name), backup); err != nil {
		return nil, fmt.Errorf("could not preserve legacy vault at %s: %w", backup, err)
	}
	if err := s.publishVault(VaultInfo{ID: id, Name: legacy.Name}); err != nil {
		return nil, fmt.Errorf("migration is complete but vault metadata could not be published; backup: %s, new vault: %s: %w", backup, vaultRootPath(s, id), err)
	}
	return verified, nil
}

// findCompleteMigrationStage reuses a valid unpublished stage for this vault.
func (s *OSStorage) findCompleteMigrationStage(legacy *paw.Vault, keyData []byte) (*model.Vault, error) {
	entries, err := os.ReadDir(storageRootPath(s))
	if err != nil {
		return nil, err
	}
	var complete *model.Vault
	for _, entry := range entries {
		id, ok := idFromFilesystemName(entry.Name())
		if !entry.IsDir() || !ok {
			continue
		}
		stageDir := filepath.Join(storageRootPath(s), entry.Name())
		if _, err := os.Stat(vaultMetadataPath(s, id)); err == nil || !os.IsNotExist(err) {
			continue
		}
		stageKey, err := os.ReadFile(keyPath(s, id))
		if err != nil || !bytes.Equal(stageKey, keyData) {
			continue
		}
		data, err := s.readVaultData(id, legacy.Key())
		if err != nil {
			if removeErr := os.RemoveAll(stageDir); removeErr != nil {
				return nil, fmt.Errorf("could not remove incomplete migration stage: %w", removeErr)
			}
			continue
		}
		stage, err := model.DecodeVault(data, legacy.Key())
		if err != nil || stage.Metadata.VaultName != legacy.Name || len(stage.Accounts[0].Items) != legacy.Size() {
			if removeErr := os.RemoveAll(stageDir); removeErr != nil {
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
			if removeErr := os.RemoveAll(stageDir); removeErr != nil {
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

// readVaultData reads and fully decrypts a vault file.
func (s *OSStorage) readVaultData(vaultID string, key *paw.Key) ([]byte, error) {
	r, err := os.Open(vaultPath(s, vaultID))
	if err != nil {
		return nil, fmt.Errorf("could not open vault: %w", err)
	}
	defer r.Close()
	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt vault: %w", err)
	}
	return data, nil
}

// LoadItem reads and validates an encrypted CXF item by ID.
func (s *OSStorage) LoadItem(vault *model.Vault, itemID string) (*model.Item, error) {
	if !vaultHasItem(vault, itemID) {
		return nil, fmt.Errorf("item is not in the vault")
	}
	r, err := os.Open(itemPath(s, vault.Accounts[0].ID, itemID))
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
func (s *OSStorage) StoreItem(vault *model.Vault, item *model.Item) error {
	return s.storeItem(vault, item)
}

// DeleteItem removes a CXF item and its vault summary.
func (s *OSStorage) DeleteItem(vault *model.Vault, itemID string) error {
	return s.deleteItem(vault, itemID)
}

// RenameVault updates the encrypted vault name and its catalog entry.
func (s *OSStorage) RenameVault(vault *model.Vault, name string) error {
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
func (s *OSStorage) storeItem(vault *model.Vault, item *model.Item) error {
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
func (s *OSStorage) deleteItem(vault *model.Vault, itemID string) error {
	updated, err := prepareItemDeleteVault(vault, itemID)
	if err != nil {
		return err
	}
	id := updated.Accounts[0].ID
	if err := os.Remove(itemPath(s, id, itemID)); err != nil {
		return fmt.Errorf("could not delete item file: %w", err)
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncrypted(vaultPath(s, id), vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was deleted: %w", err)
	}
	*vault = *updated
	return nil
}

// storeEncrypted opens a local file and checks encryption, write, and close errors.
func (s *OSStorage) storeEncrypted(path string, key *paw.Key, value any) error {
	writer, err := s.createFile(path)
	if err != nil {
		return err
	}
	return encryptFile(key, writer, value)
}

// publishVault removes an incomplete catalog entry when publication fails.
func (s *OSStorage) publishVault(info VaultInfo) error {
	// Check before cleanup too: an invalid ID must never select a path to remove.
	if !isVaultID(info.ID) {
		return fmt.Errorf("could not publish vault metadata: invalid CXF vault ID")
	}
	if err := s.storeVaultInfo(info); err != nil {
		publishErr := fmt.Errorf("could not publish vault metadata: %w", err)
		if removeErr := os.Remove(vaultMetadataPath(s, info.ID)); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return errors.Join(publishErr, fmt.Errorf("could not remove incomplete vault metadata: %w", removeErr))
		}
		return publishErr
	}
	return nil
}

// isLegacyVault reports whether decrypted vault JSON has the legacy Paw shape.
func isLegacyVault(data []byte) bool {
	var fields map[string]json.RawMessage
	if json.Unmarshal(bytes.TrimSpace(data), &fields) != nil {
		return false
	}
	for name := range fields {
		if strings.EqualFold(name, "ItemMetadata") {
			return true
		}
	}
	return false
}

// decodeModelItem decodes and validates an item stored in CXF format.
func decodeModelItem(data []byte) (*model.Item, error) {
	var item model.Item
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, fmt.Errorf("could not decode CXF item: %w", err)
	}
	return &item, nil
}

// vaultHasItem checks whether an item summary belongs to the vault.
func vaultHasItem(vault *model.Vault, itemID string) bool {
	if vault == nil || len(vault.Accounts) != 1 {
		return false
	}
	for _, item := range vault.Accounts[0].Items {
		if item.ID == itemID {
			return true
		}
	}
	return false
}

// cloneVaultAccounts copies the account and mutable slices before a vault update.
func cloneVaultAccounts(accounts []cxf.Account) []cxf.Account {
	cloned := append([]cxf.Account(nil), accounts...)
	for index := range cloned {
		cloned[index].Items = append([]cxf.Item(nil), accounts[index].Items...)
		cloned[index].Extensions = append([]json.RawMessage(nil), accounts[index].Extensions...)
	}
	return cloned
}
