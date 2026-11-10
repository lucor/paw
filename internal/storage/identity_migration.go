// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.lucor.dev/paw/internal/paw"
)

const (
	identityStageSuffix  = ".pq-stage"
	identityBackupSuffix = ".pq-backup"
	syncIdentityFile     = "sync.age"
)

// VaultKeyNeedsMigration reports whether unlocking will migrate a legacy identity.
func (s *FileStorage) VaultKeyNeedsMigration(vaultID string) (bool, error) {
	if !isVaultID(vaultID) {
		return false, fmt.Errorf("invalid vault ID")
	}
	for _, dir := range []string{vaultDirectory(vaultID), vaultDirectory(vaultID) + identityBackupSuffix} {
		root, err := s.root.OpenRoot(dir)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		_, err = root.Lstat(keyFileName)
		_ = root.Close()
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}

// loadVaultKey selects the expected identity format and completes any interrupted migration.
func (s *FileStorage) loadVaultKey(vaultID, password string) (*paw.Key, error) {
	active := vaultDirectory(vaultID)
	stage := active + identityStageSuffix
	backup := active + identityBackupSuffix
	activeExists, err := s.directoryExists(active)
	if err != nil {
		return nil, err
	}
	if !activeExists {
		backupExists, err := s.directoryExists(backup)
		if err != nil {
			return nil, err
		}
		if !backupExists {
			return nil, fmt.Errorf("vault migration is incomplete; restore its vault directory from backup")
		}
		_, err = loadIdentityAt(s, backup, keyFileName, password, false)
		if err != nil {
			return nil, fmt.Errorf("could not recover interrupted vault migration: %w", err)
		}
		// Restart from the untouched original instead of trusting partial staging.
		if err := s.root.Rename(backup, active); err != nil {
			return nil, fmt.Errorf("could not restore the original vault directory: %w", err)
		}
		if err := syncDirectory(s.root, storageRootName); err != nil {
			return nil, fmt.Errorf("could not persist recovered vault migration: %w", err)
		}
	}

	activeRoot, err := s.openVaultRoot(vaultID)
	if err != nil {
		return nil, fmt.Errorf("could not open vault: %w", err)
	}
	legacyInfo, legacyErr := activeRoot.Lstat(keyFileName)
	newInfo, newErr := activeRoot.Lstat(scryptIdentityPath())
	closeErr := activeRoot.Close()
	if err := errors.Join(closeErr, classifyPathError(legacyErr), classifyPathError(newErr)); err != nil {
		return nil, fmt.Errorf("could not inspect vault identities: %w", err)
	}
	if legacyInfo != nil && newInfo != nil {
		return nil, fmt.Errorf("vault contains both legacy and post-quantum identities; resolve the duplicate identity files before unlocking")
	}
	if newInfo != nil {
		key, err := loadIdentityAt(s, active, scryptIdentityPath(), password, true)
		if err != nil {
			return nil, err
		}
		// Authenticate the activated data before deleting its last recovery copy.
		backupExists, err := s.directoryExists(backup)
		if err != nil {
			return nil, err
		}
		if backupExists {
			root, err := s.openVaultRoot(vaultID)
			if err != nil {
				return nil, err
			}
			validationErr := validateEncryptedTree(root, ".", key)
			_ = root.Close()
			if validationErr != nil {
				return nil, fmt.Errorf("activated vault failed validation; legacy recovery copy preserved: %w", validationErr)
			}
		}
		for _, stale := range []string{stage, backup} {
			if err := s.root.RemoveAll(stale); err != nil {
				return nil, fmt.Errorf("vault unlocked, but stale migration data could not be removed: %w", err)
			}
		}
		return key, nil
	}
	if legacyInfo == nil {
		return nil, fmt.Errorf("vault has no supported identity file")
	}
	legacyKey, err := loadIdentityAt(s, active, keyFileName, password, false)
	if err != nil {
		return nil, err
	}
	if err := s.migrateVaultIdentity(active, stage, backup, password, legacyKey); err != nil {
		return nil, err
	}
	key, err := loadIdentityAt(s, active, scryptIdentityPath(), password, true)
	if err != nil {
		return nil, fmt.Errorf("vault was migrated but the new identity could not be loaded: %w", err)
	}
	if err := s.root.RemoveAll(backup); err != nil {
		return nil, fmt.Errorf("vault migrated, but its legacy recovery copy could not be removed: %w", err)
	}
	key.MarkMigrated()
	return key, nil
}

func (s *FileStorage) migrateVaultIdentity(active, stage, backup, password string, oldKey *paw.Key) error {
	backupExists, err := s.directoryExists(backup)
	if err != nil {
		return err
	}
	if backupExists {
		return fmt.Errorf("vault migration recovery directory already exists; unlock again to recover it safely")
	}
	if err := s.root.RemoveAll(stage); err != nil {
		return fmt.Errorf("could not clear previous staged migration: %w", err)
	}
	if err := s.prepareIdentityStage(active, stage, password, oldKey); err != nil {
		return err
	}
	if err := s.root.Rename(active, backup); err != nil {
		return fmt.Errorf("could not preserve the original vault before migration activation: %w", err)
	}
	if err := syncDirectory(s.root, storageRootName); err != nil {
		return fmt.Errorf("could not persist the original vault recovery copy: %w", err)
	}
	if err := s.root.Rename(stage, active); err != nil {
		if restoreErr := s.root.Rename(backup, active); restoreErr != nil {
			return errors.Join(fmt.Errorf("could not activate staged vault migration: %w", err), fmt.Errorf("could not restore the original vault path: %w", restoreErr))
		}
		return fmt.Errorf("could not activate staged vault migration: %w", err)
	}
	if err := syncDirectory(s.root, storageRootName); err != nil {
		return fmt.Errorf("could not persist migrated vault activation: %w", err)
	}
	return nil
}

func (s *FileStorage) prepareIdentityStage(source, stage, password string, oldKey *paw.Key) error {
	if err := s.root.Mkdir(stage, 0o700); err != nil {
		return fmt.Errorf("create staged vault directory: %w", err)
	}
	sourceRoot, err := s.root.OpenRoot(source)
	if err != nil {
		return fmt.Errorf("open source vault: %w", err)
	}
	defer sourceRoot.Close()
	stageRoot, err := s.root.OpenRoot(stage)
	if err != nil {
		return fmt.Errorf("open staged vault: %w", err)
	}
	defer stageRoot.Close()
	newKey, err := createMigrationIdentity(stageRoot, password)
	if err != nil {
		return err
	}
	entries, err := readRootDir(sourceRoot, ".")
	if err != nil {
		return fmt.Errorf("list source vault: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if name == keyFileName || name == identitiesDirectoryName {
			continue
		}
		if name == itemsDirectoryName || name == filesDirectoryName {
			if !entry.IsDir() {
				return fmt.Errorf("vault data directory %q is not a directory", name)
			}
			if err := migrateIdentityTree(sourceRoot, stageRoot, name, oldKey, newKey); err != nil {
				return err
			}
			continue
		}
		if name == vaultMetadataFileName || strings.HasSuffix(name, ".age") {
			if !entry.Type().IsRegular() {
				return fmt.Errorf("vault file %q is not a regular file", name)
			}
			if name == syncIdentityFile {
				return fmt.Errorf("vault contains a Sync identity that requires the legacy key; migration cannot continue safely")
			}
			if strings.HasSuffix(name, ".age") {
				if err := reencryptVaultFile(sourceRoot, stageRoot, name, oldKey, newKey); err != nil {
					return fmt.Errorf("migrate encrypted vault file %q: %w", name, err)
				}
			} else if err := copyVaultFile(sourceRoot, stageRoot, name); err != nil {
				return fmt.Errorf("copy vault metadata: %w", err)
			}
		}
	}
	if err := validateIdentityStage(stageRoot, password); err != nil {
		return err
	}
	if err := syncIdentityDirectories(stageRoot, "."); err != nil {
		return fmt.Errorf("persist staged vault: %w", err)
	}
	return nil
}

func syncIdentityDirectories(root *os.Root, dir string) error {
	entries, err := readRootDir(root, dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			path := filepath.Join(dir, entry.Name())
			if err := syncIdentityDirectories(root, path); err != nil {
				return err
			}
		}
	}
	return syncDirectory(root, dir)
}

func migrateIdentityTree(source, stage *os.Root, dir string, oldKey, newKey *paw.Key) error {
	entries, err := readRootDir(source, dir)
	if err != nil {
		return fmt.Errorf("list vault directory %q: %w", dir, err)
	}
	if err := stage.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create staged vault directory %q: %w", dir, err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".paw-") || strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			if entry.Name() != filesDirectoryName && !isFilesystemVaultID(entry.Name()) {
				continue
			}
			if err := migrateIdentityTree(source, stage, path, oldKey, newKey); err != nil {
				return err
			}
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".age") {
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("encrypted vault file %q is not a regular file", path)
		}
		if err := reencryptVaultFile(source, stage, path, oldKey, newKey); err != nil {
			return fmt.Errorf("migrate encrypted vault file %q: %w", path, err)
		}
	}
	return nil
}

func createMigrationIdentity(root *os.Root, password string) (*paw.Key, error) {
	if err := root.MkdirAll(identitiesDirectoryName, 0o700); err != nil {
		return nil, fmt.Errorf("create staged identity directory: %w", err)
	}
	var identity *paw.Key
	path := scryptIdentityPath()
	if err := atomicWrite(root, path, func(w io.Writer) error {
		var err error
		identity, err = paw.MakeKey(password, w)
		return err
	}); err != nil {
		return nil, fmt.Errorf("write staged post-quantum identity: %w", err)
	}
	return identity, nil
}

func reencryptVaultFile(source, stage *os.Root, path string, oldKey, newKey *paw.Key) error {
	r, err := source.Open(path)
	if err != nil {
		return err
	}
	decrypted, err := oldKey.Decrypt(r)
	if err != nil {
		return errors.Join(err, r.Close())
	}
	err = atomicWrite(stage, path, func(w io.Writer) error {
		encrypted, err := newKey.Encrypt(w)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(encrypted, decrypted)
		return errors.Join(copyErr, encrypted.Close())
	})
	return errors.Join(err, r.Close())
}

func copyVaultFile(source, stage *os.Root, path string) error {
	r, err := source.Open(path)
	if err != nil {
		return err
	}
	defer r.Close()
	return atomicWrite(stage, path, func(w io.Writer) error {
		_, err := io.Copy(w, r)
		return err
	})
}

func validateIdentityStage(root *os.Root, password string) error {
	vaultInfo, err := root.Lstat(vaultFileName)
	if err != nil || !vaultInfo.Mode().IsRegular() || vaultInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("staged encrypted vault file is missing or invalid")
	}
	key, err := loadKeyFromRoot(root, scryptIdentityPath(), password, true)
	if err != nil {
		return fmt.Errorf("staged post-quantum identity is invalid: %w", err)
	}
	return validateEncryptedTree(root, ".", key)
}

func validateEncryptedTree(root *os.Root, dir string, key *paw.Key) error {
	entries, err := readRootDir(root, dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := entry.Name()
		if dir != "." {
			path = filepath.Join(dir, path)
		}
		if entry.IsDir() {
			if err := validateEncryptedTree(root, path, key); err != nil {
				return err
			}
			continue
		}
		if !entry.Type().IsRegular() || !strings.HasSuffix(path, ".age") || path == syncIdentityFile || path == keyFileName || path == scryptIdentityPath() {
			continue
		}
		file, err := root.Open(path)
		if err != nil {
			return err
		}
		decrypted, decryptErr := key.Decrypt(file)
		if decryptErr == nil {
			_, decryptErr = io.Copy(io.Discard, decrypted)
		}
		decryptErr = errors.Join(decryptErr, file.Close())
		if decryptErr != nil {
			return fmt.Errorf("staged encrypted file %q failed validation: %w", path, decryptErr)
		}
	}
	return nil
}

func loadIdentityAt(s *FileStorage, dir, path, password string, hybrid bool) (*paw.Key, error) {
	root, err := s.root.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("could not open vault identity directory: %w", err)
	}
	defer root.Close()
	return loadKeyFromRoot(root, path, password, hybrid)
}

func loadKeyFromRoot(root *os.Root, path, password string, hybrid bool) (*paw.Key, error) {
	info, err := root.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("could not inspect vault identity: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("vault identity is not a regular file")
	}
	reader, err := root.Open(path)
	if err != nil {
		return nil, fmt.Errorf("could not read vault identity: %w", err)
	}
	defer reader.Close()
	if hybrid {
		return paw.LoadKey(password, reader)
	}
	return paw.LoadLegacyKey(password, reader)
}

func (s *FileStorage) directoryExists(name string) (bool, error) {
	info, err := s.root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("vault migration path %q is not a local directory", name)
	}
	return true, nil
}

func classifyPathError(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readRootDir(root *os.Root, name string) ([]os.DirEntry, error) {
	dir, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	return dir.ReadDir(-1)
}
