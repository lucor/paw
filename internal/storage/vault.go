// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// DeleteVault removes a vault directory by its canonical ID.
func (s *FileStorage) DeleteVault(vaultID string) error {
	if !isVaultID(vaultID) {
		return fmt.Errorf("invalid vault ID")
	}
	if err := s.root.RemoveAll(vaultDirectory(vaultID)); err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultKey decrypts the vault key using its canonical ID and password.
func (s *FileStorage) LoadVaultKey(vaultID, password string) (*paw.Key, error) {
	if !isVaultID(vaultID) {
		return nil, fmt.Errorf("invalid vault ID")
	}
	root, err := s.openVaultRoot(vaultID)
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer root.Close()
	r, err := root.Open(keyFileName)
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return paw.LoadKey(password, r)
}

// storeVaultInfo writes the public name used to list a CXF vault.
func (s *FileStorage) storeVaultInfo(info model.VaultInfo) error {
	if !isVaultID(info.ID) {
		return fmt.Errorf("invalid CXF vault ID")
	}
	if err := s.root.MkdirAll(vaultDirectory(info.ID), 0o700); err != nil {
		return fmt.Errorf("could not create vault directory: %w", err)
	}
	root, err := s.openVaultRoot(info.ID)
	if err != nil {
		return fmt.Errorf("could not open vault directory: %w", err)
	}
	defer root.Close()
	return atomicWrite(root, vaultMetadataFileName, func(w io.Writer) error { return writeVaultMetadata(w, info) })
}

// Vaults returns vault IDs and display names for current vaults.
func (s *FileStorage) Vaults() ([]model.VaultInfo, error) {
	entries, err := s.readDir(storageRootName)
	if err != nil {
		return nil, err
	}
	vaults := []model.VaultInfo{}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasSuffix(entry.Name(), vaultBackupSuffix) {
			continue
		}
		if !isFilesystemVaultID(entry.Name()) {
			continue
		}
		id, _ := idFromFilesystemName(entry.Name())
		vaultRoot, err := s.openVaultRoot(id)
		if err != nil {
			return nil, fmt.Errorf("could not open vault directory: %w", err)
		}
		data, readErr := vaultRoot.ReadFile(vaultMetadataFileName)
		closeErr := vaultRoot.Close()
		err = errors.Join(readErr, closeErr)
		if errors.Is(err, os.ErrNotExist) {
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

// CreateVault saves a new CXF vault before publishing its catalog entry.
func (s *FileStorage) CreateVault(name, password string, appearances ...model.VaultAppearance) (*model.Vault, error) {
	vault, keyData, err := prepareVault(name, password)
	if err != nil {
		return nil, err
	}
	id := vault.Accounts[0].ID
	if err := s.root.Mkdir(vaultDirectory(id), 0o700); err != nil {
		return nil, fmt.Errorf("could not create vault directory: %w", err)
	}
	vaultRoot, err := s.openVaultRoot(id)
	if err != nil {
		return nil, err
	}
	defer vaultRoot.Close()
	if err := vaultRoot.Mkdir(itemsDirectoryName, 0o700); err != nil {
		return nil, fmt.Errorf("could not create item storage directory: %w", err)
	}
	if err := atomicWrite(vaultRoot, keyFileName, func(w io.Writer) error {
		_, err := w.Write(keyData)
		return err
	}); err != nil {
		return nil, fmt.Errorf("could not store vault key: %w", err)
	}
	if err := s.storeEncryptedInRoot(vaultRoot, vaultFileName, vault.Key(), vault); err != nil {
		return nil, fmt.Errorf("could not store vault: %w", err)
	}
	info := model.VaultInfo{ID: id, Name: name, Appearance: requestedVaultAppearance(appearances)}
	if err := atomicWrite(vaultRoot, vaultMetadataFileName, func(w io.Writer) error { return writeVaultMetadata(w, info) }); err != nil {
		return nil, fmt.Errorf("could not publish vault metadata: %w", err)
	}
	return vault, nil
}

// LoadVault reads and decrypts a vault.
func (s *FileStorage) LoadVault(vaultID string, key *paw.Key) (*model.Vault, error) {
	if !isVaultID(vaultID) {
		return nil, fmt.Errorf("invalid vault ID")
	}
	data, err := s.readVaultData(vaultID, key)
	if err != nil {
		return nil, err
	}
	vault, err := model.DecodeVault(data, key)
	if err != nil {
		return nil, fmt.Errorf("could not decode CXF vault: %w", err)
	}
	if vault.Accounts[0].ID != vaultID {
		return nil, fmt.Errorf("vault ID does not match requested ID")
	}
	if err := s.migrateItemLayout(vault); err != nil {
		return nil, fmt.Errorf("could not migrate vault item storage: %w", err)
	}
	return vault, nil
}

// readVaultData reads and fully decrypts a vault file.
func (s *FileStorage) readVaultData(vaultID string, key *paw.Key) ([]byte, error) {
	root, err := s.openVaultRoot(vaultID)
	if err != nil {
		return nil, fmt.Errorf("could not open vault: %w", err)
	}
	defer root.Close()
	return readEncryptedRoot(root, vaultFileName, key)
}

// RenameVault updates the encrypted vault name and its catalog entry.
func (s *FileStorage) RenameVault(vault *model.Vault, name string) error {
	return s.UpdateVaultIdentity(vault, name, s.vaultAppearance(vault.Accounts[0].ID))
}

// UpdateVaultIdentity updates the encrypted name and public appearance metadata.
func (s *FileStorage) UpdateVaultIdentity(vault *model.Vault, name string, appearance model.VaultAppearance) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("vault name is required")
	}
	if name == vault.Metadata.VaultName {
		return s.storeVaultInfo(model.VaultInfo{ID: vault.Accounts[0].ID, Name: name, Appearance: model.NormalizeVaultAppearance(appearance)})
	}
	updated := *vault
	updated.Metadata.VaultName = name
	updated.Accounts = cloneVaultAccounts(vault.Accounts)
	updated.Timestamp = uint64(time.Now().Unix())
	if err := updated.Validate(); err != nil {
		return fmt.Errorf("cannot rename vault: %w", err)
	}
	id := updated.Accounts[0].ID
	root, err := s.openVaultRoot(id)
	if err != nil {
		return fmt.Errorf("could not open vault: %w", err)
	}
	defer root.Close()
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), &updated); err != nil {
		return fmt.Errorf("could not store renamed vault: %w", err)
	}
	if err := atomicWrite(root, vaultMetadataFileName, func(w io.Writer) error {
		return writeVaultMetadata(w, model.VaultInfo{ID: id, Name: name, Appearance: model.NormalizeVaultAppearance(appearance)})
	}); err != nil {
		return fmt.Errorf("could not update vault catalog: %w", err)
	}
	*vault = updated
	return nil
}

// cloneVaultAccounts copies the account and mutable slices before a vault update.
func cloneVaultAccounts(accounts []cxf.Account) []cxf.Account {
	cloned := slices.Clone(accounts)
	for index := range cloned {
		cloned[index].Items = slices.Clone(accounts[index].Items)
		cloned[index].Extensions = slices.Clone(accounts[index].Extensions)
	}
	return cloned
}

// prepareVault creates a CXF vault and its password-encrypted key contents.
func prepareVault(name, password string) (*model.Vault, []byte, error) {
	var keyData bytes.Buffer
	key, err := paw.MakeKey(password, &keyData)
	if err != nil {
		return nil, nil, fmt.Errorf("could not create vault key: %w", err)
	}
	vault, err := model.NewVault(key, name)
	if err != nil {
		return nil, nil, err
	}
	return vault, keyData.Bytes(), nil
}

// vaultInfoFromFilesystemName decodes a vault directory name and its metadata.
func vaultInfoFromFilesystemName(name string, data []byte) (model.VaultInfo, error) {
	id, ok := idFromFilesystemName(name)
	if !ok {
		return model.VaultInfo{}, fmt.Errorf("invalid vault directory ID")
	}
	return vaultInfoFromMetadata(id, data)
}

// vaultInfoFromMetadata decodes a CXF vault's listing metadata.
func vaultInfoFromMetadata(id string, data []byte) (model.VaultInfo, error) {
	if !isVaultID(id) {
		return model.VaultInfo{}, fmt.Errorf("invalid CXF vault ID")
	}
	metadata, err := decodeVaultMetadata(data)
	if err != nil {
		return model.VaultInfo{}, err
	}
	return model.VaultInfo{ID: id, Name: metadata.Name, Appearance: model.NormalizeVaultAppearance(model.VaultAppearance{Icon: metadata.Icon, Color: metadata.Color})}, nil
}

// requestedVaultAppearance returns the normalized requested appearance or its default.
func requestedVaultAppearance(appearances []model.VaultAppearance) model.VaultAppearance {
	if len(appearances) == 0 {
		return model.DefaultVaultAppearance()
	}
	return model.NormalizeVaultAppearance(appearances[0])
}

// vaultAppearance looks up a vault's public appearance and falls back to defaults.
func (s *FileStorage) vaultAppearance(id string) model.VaultAppearance {
	vaults, err := s.Vaults()
	if err != nil {
		return model.DefaultVaultAppearance()
	}
	for _, vault := range vaults {
		if vault.ID == id {
			return vault.Appearance
		}
	}
	return model.DefaultVaultAppearance()
}
