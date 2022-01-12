package paw

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	fyneStorage "fyne.io/fyne/v2/storage"
)

// Declare conformity to Item interface
var _ Storage = (*FyneStorage)(nil)

type FyneStorage struct {
	fs fyne.Storage
}

func NewFyneStorage(storage fyne.Storage) (Storage, error) {
	s := &FyneStorage{fs: storage}

	// check if the vaults root URI exists, if not try to create one.
	storageRootURI := fyneStorage.NewFileURI(storageRootPath(s))
	exists, err := fyneStorage.Exists(storageRootURI)
	if err != nil {
		return nil, fmt.Errorf("could not check for the existence of the Paw vaults root folder: %w", err)
	}
	if exists {
		return s, nil
	}

	migrated, err := s.migrateDeprecatedRootStorage()
	if migrated {
		if err != nil {
			return nil, fmt.Errorf("found deprecated 'vaults' storage folder but was unable to rename into 'storage': %w", err)
		}
		return s, nil
	}

	err = fyneStorage.CreateListable(storageRootURI)
	if err != nil {
		return nil, fmt.Errorf("could not create the Paw vaults root folder: %w", err)
	}
	return s, nil
}

func (s *FyneStorage) Root() string {
	return s.fs.RootURI().Path()
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *FyneStorage) CreateVaultKey(name string, password string) (*Key, error) {
	root := fyneStorage.NewFileURI(vaultRootPath(s, name))
	exists, err := fyneStorage.Exists(root)
	if err != nil {
		return nil, fmt.Errorf("could not check for vault URI: %w", err)
	}
	if !exists {
		err = fyneStorage.CreateListable(root)
		if err != nil {
			return nil, fmt.Errorf("could not create vault root URI: %w", err)
		}
	}

	keyURI := fyneStorage.NewFileURI(keyPath(s, name))
	exists, err = fyneStorage.Exists(keyURI)
	if err != nil {
		return nil, fmt.Errorf("could not check for the existence of the key: %w", err)
	}
	if exists {
		return nil, errors.New("key with the same name already exists")
	}

	writer, err := fyneStorage.Writer(keyURI)
	if err != nil {
		return nil, fmt.Errorf("could not create writer for the key URI: %w", err)
	}
	defer writer.Close()

	key, err := MakeKey(password, writer)
	if err != nil {
		return nil, fmt.Errorf("could not create the vault key: %w", err)
	}

	return key, nil
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *FyneStorage) CreateVault(name string, key *Key) (*Vault, error) {
	root := fyneStorage.NewFileURI(vaultRootPath(s, name))
	exists, err := fyneStorage.Exists(root)
	if err != nil {
		return nil, fmt.Errorf("could not check for vault URI: %w", err)
	}
	if !exists {
		err = fyneStorage.CreateListable(root)
		if err != nil {
			return nil, fmt.Errorf("could not create vault root URI: %w", err)
		}
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
	vaultURI := fyneStorage.NewFileURI(vaultPath(s, name))
	err := fyneStorage.Delete(vaultURI)
	if err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultIdentity returns a vault decrypting from the underlying storage
func (s *FyneStorage) LoadVaultKey(name string, password string) (*Key, error) {
	keyURI := fyneStorage.NewFileURI(keyPath(s, name))
	reader, err := fyneStorage.Reader(keyURI)
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer reader.Close()
	return LoadKey(password, reader)
}

// LoadVault returns a vault decrypting from the underlying storage
func (s *FyneStorage) LoadVault(name string, password string) (*Vault, error) {
	key, err := s.LoadVaultKey(name, password)
	if err != nil {
		return nil, fmt.Errorf("could not load the vault key: %w", err)
	}
	vault := NewVault(key, name)
	vaultURI := fyneStorage.NewFileURI(vaultPath(s, name))
	r, err := fyneStorage.Reader(vaultURI)
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	err = decrypt(vault.key, r, vault)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the vault: %w", err)
	}
	return vault, nil
}

// StoreVault encrypts and stores the vault into the underlying storage
func (s *FyneStorage) StoreVault(vault *Vault) error {
	vaultURI := fyneStorage.NewFileURI(vaultPath(s, vault.Name))
	w, err := fyneStorage.Writer(vaultURI)
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
	itemURI := fyneStorage.NewFileURI(itemPath(s, vault.Name, item.ID()))
	err := fyneStorage.Delete(itemURI)
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
	}
	itemURI := fyneStorage.NewFileURI(itemPath(s, vault.Name, itemMetadata.ID()))
	r, err := fyneStorage.Reader(itemURI)
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
	itemURI := fyneStorage.NewFileURI(itemPath(s, vault.Name, item.ID()))

	w, err := fyneStorage.Writer(itemURI)
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
	storageRootURI := fyneStorage.NewFileURI(storageRootPath(s))
	vaultsURI, err := fyneStorage.List(storageRootURI)
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

// func (s *FyneStorage) encrypt(key *Key, uri fyne.URI, v interface{}) error {
// 	writer, err := fyneStorage.Writer(uri)
// 	if err != nil {
// 		return fmt.Errorf("could not create writer: %w", err)
// 	}
// 	defer writer.Close()

// 	encWriter, err := key.Encrypt(writer)
// 	if err != nil {
// 		return fmt.Errorf("could not create encrypted writer for URI: %w", err)
// 	}
// 	defer encWriter.Close()

// 	err = json.NewEncoder(encWriter).Encode(v)
// 	if err != nil {
// 		return fmt.Errorf("could not encode data for URI: %w", err)
// 	}

// 	return nil
// }

// migrateDeprecatedRootStorage migrates the deprecated 'vaults' storage folder to new one
func (s *FyneStorage) migrateDeprecatedRootStorage() (bool, error) {
	src := filepath.Join(s.Root(), "vaults")
	_, err := os.Stat(src)
	if os.IsNotExist(err) {
		return false, nil
	}
	dest := storageRootPath(s)
	err = os.Rename(src, dest)
	return true, err
}
