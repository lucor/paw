// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

import (
	"encoding/json"
	"errors"
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"
)

// Declare conformity to Item interface
var _ Storage = (*FyneStorage)(nil)

type FyneStorage struct {
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

func (s *FyneStorage) Root() string {
	return s.Storage.RootURI().Path()
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *FyneStorage) CreateVaultKey(name string, password string) (*Key, error) {
	err := s.mkdirIfNotExists(vaultRootPath(s, name))
	if err != nil {
		return nil, fmt.Errorf("could not create vault root dir: %w", err)
	}

	keyFile := keyPath(s, name)
	if s.isExist(keyFile) {
		return nil, errors.New("key with the same name already exists")
	}

	w, err := s.createFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("could not create writer for the key file: %w", err)
	}
	defer w.Close()

	key, err := MakeKey(password, w)
	if err != nil {
		return nil, fmt.Errorf("could not create the vault key file: %w", err)
	}

	return key, nil
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *FyneStorage) CreateVault(name string, key *Key) (*Vault, error) {
	err := s.mkdirIfNotExists(vaultRootPath(s, name))
	if err != nil {
		return nil, fmt.Errorf("could not create vault root dir: %w", err)
	}

	vault := NewVault(key, name)
	err = s.StoreVault(vault)
	if err != nil {
		return nil, err
	}
	return vault, nil
}

// DeleteVault delete the specified vault
func (s *FyneStorage) DeleteVault(name string) error {
	return fmt.Errorf("TODO")
	// err := os.RemoveAll(vaultPath(s, name))
	// if err != nil {
	// 	return fmt.Errorf("could not delete the vault: %w", err)
	// }
	// return nil
}

// LoadVaultIdentity returns a vault decrypting from the underlying storage
func (s *FyneStorage) LoadVaultKey(name string, password string) (*Key, error) {
	keyFile := keyPath(s, name)
	r, err := storage.Reader(storage.NewFileURI(keyFile))
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return LoadKey(password, r)
}

// LoadVault returns a vault decrypting from the underlying storage
func (s *FyneStorage) LoadVault(name string, key *Key) (*Vault, error) {
	vault := NewVault(key, name)
	vaultFile := vaultPath(s, name)

	r, err := storage.Reader(storage.NewFileURI(vaultFile))
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	err = decrypt(key, r, vault)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the vault: %w", err)
	}
	return vault, nil
}

// StoreVault encrypts and stores the vault into the underlying storage
func (s *FyneStorage) StoreVault(vault *Vault) error {
	vaultFile := vaultPath(s, vault.Name)
	w, err := s.createFile(vaultFile)
	if err != nil {
		return fmt.Errorf("could not create writer: %w", err)
	}
	defer w.Close()

	err = encrypt(vault.key, w, vault)
	if err != nil {
		return fmt.Errorf("could not encrypt and store the vault: %w", err)
	}
	return nil
}

// DeleteItem delete the item from the specified vaultName
func (s *FyneStorage) DeleteItem(vault *Vault, item Item) error {
	itemFile := itemPath(s, vault.Name, item.ID())
	err := storage.Delete(storage.NewFileURI(itemFile))
	if err != nil {
		return fmt.Errorf("could not delete the item: %w", err)
	}
	return s.StoreVault(vault)
}

// LoadItem returns a item from the vault decrypting from the underlying storage
func (s *FyneStorage) LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error) {
	var item Item
	switch itemMetadata.Type {
	case NoteItemType:
		item = &Note{}
	case PasswordItemType:
		item = &Password{}
	case LoginItemType:
		item = &Login{}
	case SSHKeyItemType:
		item = &SSHKey{}
	}

	itemFile := itemPath(s, vault.Name, itemMetadata.ID())
	r, err := storage.Reader(storage.NewFileURI(itemFile))
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	err = decrypt(vault.key, r, item)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the item: %w", err)
	}
	return item, nil
}

// StoreItem encrypts and encrypts and stores the item into the specified vault
func (s *FyneStorage) StoreItem(vault *Vault, item Item) error {
	itemFile := itemPath(s, vault.Name, item.ID())
	w, err := s.createFile(itemFile)
	if err != nil {
		return fmt.Errorf("could not create writer: %w", err)
	}
	defer w.Close()

	err = encrypt(vault.key, w, item)
	if err != nil {
		return fmt.Errorf("could not encrypt and store the item: %w", err)
	}
	return s.StoreVault(vault)
}

// Vaults returns the list of vault names from the storage
func (s *FyneStorage) Vaults() ([]string, error) {
	root := storage.NewFileURI(storageRootPath(s))
	vaults := []string{}

	dirEntries, err := storage.List(root)
	if err != nil {
		return nil, err
	}

	for _, dirEntry := range dirEntries {
		if ok, err := storage.CanList(dirEntry); !ok {
			if err != nil {
				fyne.LogError("could not list dir entry", err)
			}
			continue
		}
		vaults = append(vaults, dirEntry.Name())
	}

	return vaults, nil
}

// LoadConfig load the configuration from the underlying storage
func (s *FyneStorage) LoadConfig() (*Config, error) {
	configFile := configPath(s)
	if !s.isExist(configFile) {
		return newDefaultConfig(), nil
	}
	r, err := storage.Reader(storage.NewFileURI(configFile))
	if err != nil {
		return newDefaultConfig(), fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	config := &Config{}
	err = json.NewDecoder(r).Decode(config)
	if err != nil {
		return newDefaultConfig(), err
	}
	return config, nil
}

// StoreConfig store the configuration into the underlying storage
func (s *FyneStorage) StoreConfig(config *Config) error {
	configFile := configPath(s)
	w, err := s.createFile(configFile)
	if err != nil {
		return err
	}
	defer w.Close()
	return json.NewEncoder(w).Encode(config)
}

// SocketAgentPath return the socket agent path
func (s *FyneStorage) SocketAgentPath() string {
	return socketAgentPath(s)
}

// LockFilePath return the lock file path
func (s *FyneStorage) LockFilePath() string {
	return lockFilePath(s)
}

func (s *FyneStorage) isExist(path string) bool {
	ok, _ := storage.Exists(storage.NewFileURI(path))
	return ok
}

func (s *FyneStorage) mkdirIfNotExists(path string) error {
	if s.isExist(path) {
		return nil
	}
	return storage.CreateListable(storage.NewFileURI(path))
}

func (s *FyneStorage) createFile(name string) (fyne.URIWriteCloser, error) {
	return storage.Writer(storage.NewFileURI(name))
}
