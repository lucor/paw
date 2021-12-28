package paw

import (
	"encoding/gob"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	fyneStorage "fyne.io/fyne/v2/storage"
)

type Storage struct {
	fyne.Storage
}

func NewStorage(storage fyne.Storage) (*Storage, error) {
	s := &Storage{Storage: storage}

	// check if the vaults root URI exists, if not try to create one.
	exists, err := fyneStorage.Exists(s.vaultsRootURI())
	if err != nil {
		return nil, fmt.Errorf("could not check for the existence of the Paw vaults root folder: %w", err)
	}
	if !exists {
		err = fyneStorage.CreateListable(s.vaultsRootURI())
		if err != nil {
			return nil, fmt.Errorf("could not create the Paw vaults root folder: %w", err)
		}
	}
	return s, nil
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *Storage) CreateVault(key *Key, name string) (*Vault, error) {
	vaultURI := s.vaultURI(name)
	exists, err := fyneStorage.Exists(vaultURI)
	if err != nil {
		return nil, fmt.Errorf("could not check for the existence of the vault: %w", err)
	}
	if exists {
		return nil, errors.New("vault with the same name already exists")
	}

	vault := NewVault(key, name)
	err = s.StoreVault(vault)
	if err != nil {
		return nil, err
	}
	return vault, nil
}

// DeleteVault delete the specified vault
func (s *Storage) DeleteVault(name string) error {
	err := fyneStorage.Delete(s.vaultURI(name))
	if err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVault returns a vault decrypting from the underlying storage
func (s *Storage) LoadVault(key *Key, name string) (*Vault, error) {
	vault := NewVault(key, name)
	err := s.decrypt(key, s.vaultURI(name), vault)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the vault: %w", err)
	}
	return vault, nil
}

// StoreVault encrypts and stores the vault into the underlying storage
func (s *Storage) StoreVault(vault *Vault) error {
	err := s.encrypt(vault.key, s.vaultURI(vault.Name), vault)
	if err != nil {
		return fmt.Errorf("could not encrypt and store the vault: %w", err)
	}
	return nil
}

// DeleteItem delete the item from the specified vaultName
func (s *Storage) DeleteItem(vault *Vault, item Item) error {
	err := fyneStorage.Delete(s.itemURI(vault.Name, item.ID()))
	if err != nil {
		return fmt.Errorf("could not delete the item: %w", err)
	}
	return s.StoreVault(vault)
}

// LoadItem returns a item from the vault decrypting from the underlying storage
func (s *Storage) LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error) {
	var item Item
	switch itemMetadata.Type {
	case NoteItemType:
		item = &Note{}
	case PasswordItemType:
		item = &Password{}
	case WebsiteItemType:
		item = &Website{}
	}
	err := s.decrypt(vault.key, s.itemURI(vault.Name, itemMetadata.ID()), item)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the item: %w", err)
	}
	return item, nil
}

// StoreItem encrypts and encrypts and stores the item into the specified vault
func (s *Storage) StoreItem(vault *Vault, item Item) error {
	err := s.encrypt(vault.key, s.itemURI(vault.Name, item.ID()), item)
	if err != nil {
		return fmt.Errorf("could not encrypt and store the item: %w", err)
	}
	return s.StoreVault(vault)
}

// Vaults returns the list of vault names from the storage
func (s *Storage) Vaults() ([]string, error) {
	vaultsURI, err := fyneStorage.List(s.vaultsRootURI())
	if err != nil {
		return nil, err
	}

	vaults := []string{}
	for _, vaultURI := range vaultsURI {
		vaults = append(vaults, vaultURI.Name())
	}

	sort.Strings(vaults)
	return vaults, nil
}

func (s *Storage) encrypt(key *Key, uri fyne.URI, v interface{}) error {
	root, err := fyneStorage.Parent(uri)
	if err != nil {
		return fmt.Errorf("could not retrieve parent for URI: %w", err)
	}

	exists, err := fyneStorage.Exists(root)
	if err != nil {
		return fmt.Errorf("could not check parent for URI: %w", err)
	}
	if !exists {
		err = fyneStorage.CreateListable(root)
		if err != nil {
			return fmt.Errorf("could not create parent for URI: %w", err)
		}
	}

	writer, err := fyneStorage.Writer(uri)
	if err != nil {
		return fmt.Errorf("could not create writer for URI: %w", err)
	}
	defer writer.Close()

	encWriter, err := key.Encrypt(writer)
	if err != nil {
		return fmt.Errorf("could not create encrypted writer for URI: %w", err)
	}
	defer encWriter.Close()

	err = gob.NewEncoder(encWriter).Encode(v)
	if err != nil {
		return fmt.Errorf("could not encode data for URI: %w", err)
	}

	return nil
}

func (s *Storage) decrypt(key *Key, uri fyne.URI, v interface{}) error {
	reader, err := fyneStorage.Reader(uri)
	if err != nil {
		return fmt.Errorf("could not read URI: %w", err)
	}
	defer reader.Close()

	encReader, err := key.Decrypt(reader)
	if err != nil {
		return fmt.Errorf("could not decrypt URI content: %w", err)
	}

	err = gob.NewDecoder(encReader).Decode(v)
	if err != nil {
		return fmt.Errorf("could not decode URI content: %w", err)
	}
	return nil
}

func (s *Storage) pawRootURI() fyne.URI {
	return fyneStorage.NewFileURI(filepath.Join(s.Storage.RootURI().Path()))
}

func (s *Storage) vaultsRootURI() fyne.URI {
	return fyneStorage.NewFileURI(filepath.Join(s.pawRootURI().Path(), "vaults"))
}

func (s *Storage) vaultRootURI(name string) fyne.URI {
	return fyneStorage.NewFileURI(filepath.Join(s.vaultsRootURI().Path(), name))
}

func (s *Storage) vaultURI(name string) fyne.URI {
	return fyneStorage.NewFileURI(filepath.Join(s.vaultRootURI(name).Path(), "vault.age"))
}

func (s *Storage) itemURI(vaultName string, itemID string) fyne.URI {
	vaultPath := s.vaultRootURI(vaultName).Path()
	itemFileName := fmt.Sprintf("%s.age", itemID)
	return fyneStorage.NewFileURI(filepath.Join(vaultPath, itemFileName))
}
