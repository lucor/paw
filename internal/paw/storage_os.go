package paw

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// Declare conformity to Item interface
var _ Storage = (*OSStorage)(nil)

type OSStorage struct {
	root string
}

// NewOSStorage returns an OS Storage implementation rooted at os.UserConfigDir()
func NewOSStorage() (Storage, error) {
	urd, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("could not get the default root directory to use for user-specific configuration data: %w", err)
	}
	return NewOSStorageRooted(urd)
}

// NewOSStorageRooted returns an OS Storage implementation rooted at root
func NewOSStorageRooted(root string) (Storage, error) {

	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("storage root must be an absolute path, got %s", root)
	}

	// Fyne does not allow to customize the root for a storage
	// so we'll use the same
	storageRoot := filepath.Join(root, ".paw")

	s := &OSStorage{root: storageRoot}

	migrated, err := s.migrateDeprecatedRootStorage()
	if migrated {
		if err != nil {
			return nil, fmt.Errorf("found deprecated storage but was unable to move to new location: %w", err)
		}
		return s, nil
	}

	err = s.mkdirIfNotExists(storageRootPath(s))
	return s, err
}

func (s *OSStorage) Root() string {
	return s.root
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (s *OSStorage) CreateVaultKey(name string, password string) (*Key, error) {
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
func (s *OSStorage) CreateVault(name string, key *Key) (*Vault, error) {
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
func (s *OSStorage) DeleteVault(name string) error {
	err := os.RemoveAll(vaultPath(s, name))
	if err != nil {
		return fmt.Errorf("could not delete the vault: %w", err)
	}
	return nil
}

// LoadVaultIdentity returns a vault decrypting from the underlying storage
func (s *OSStorage) LoadVaultKey(name string, password string) (*Key, error) {
	keyFile := keyPath(s, name)
	r, err := os.Open(keyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read URI: %w", err)
	}
	defer r.Close()
	return LoadKey(password, r)
}

// LoadVault returns a vault decrypting from the underlying storage
func (s *OSStorage) LoadVault(name string, password string) (*Vault, error) {
	key, err := s.LoadVaultKey(name, password)
	if err != nil {
		return nil, fmt.Errorf("could not load the vault key: %w", err)
	}
	vault := NewVault(key, name)
	vaultFile := vaultPath(s, name)

	r, err := os.Open(vaultFile)
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
func (s *OSStorage) StoreVault(vault *Vault) error {
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
func (s *OSStorage) DeleteItem(vault *Vault, item Item) error {
	itemFile := itemPath(s, vault.Name, item.ID())
	err := os.Remove(itemFile)
	if err != nil {
		return fmt.Errorf("could not delete the item: %w", err)
	}
	return s.StoreVault(vault)
}

// LoadItem returns a item from the vault decrypting from the underlying storage
func (s *OSStorage) LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error) {
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
	r, err := os.Open(itemFile)
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
func (s *OSStorage) StoreItem(vault *Vault, item Item) error {
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
func (s *OSStorage) Vaults() ([]string, error) {
	root := storageRootPath(s)
	dirEntries, err := ioutil.ReadDir(root)
	if err != nil {
		return nil, err
	}

	vaults := []string{}
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() {
			continue
		}
		vaults = append(vaults, dirEntry.Name())
	}

	return vaults, nil
}

func (s *OSStorage) isExist(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

func (s *OSStorage) mkdirIfNotExists(path string) error {
	if s.isExist(path) {
		return nil
	}
	return os.MkdirAll(path, 0700)
}

func (s *OSStorage) createFile(name string) (*os.File, error) {
	return os.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
}

// migrateDeprecatedRootStorage migrates the deprecated 'vaults' storage folder to new one
func (s *OSStorage) migrateDeprecatedRootStorage() (bool, error) {
	oldRoot, err := os.UserConfigDir()
	if err != nil {
		return false, nil
	}

	src := filepath.Join(oldRoot, "fyne", ID, "vaults")
	_, err = os.Stat(src)
	if os.IsNotExist(err) {
		return false, nil
	}
	dest := storageRootPath(s)
	err = os.Rename(src, dest)
	return true, err
}
