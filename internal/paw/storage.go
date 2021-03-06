package paw

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

const (
	storageRootName = "storage"
	keyFileName     = "key.age"
	vaultFileName   = "vault.age"
)

type Storage interface {
	Root() string
	VaultStorage
	ItemStorage
}
type VaultStorage interface {
	// CreateVault encrypts and stores an empty vault into the underlying storage.
	CreateVault(name string, key *Key) (*Vault, error)
	// LoadVaultKey creates and stores a Key used to encrypt and decrypt the vault data
	// The file containing the key is encrypted using the provided password
	CreateVaultKey(name string, password string) (*Key, error)
	// DeleteVault delete the specified vault
	DeleteVault(name string) error
	// LoadVault returns a vault decrypting from the underlying storage
	LoadVault(name string, password string) (*Vault, error)
	// LoadVaultKey returns the Key used to encrypt and decrypt the vault data
	LoadVaultKey(name string, password string) (*Key, error)
	// StoreVault encrypts and stores the vault into the underlying storage
	StoreVault(vault *Vault) error
	// Vaults returns the list of vault names from the storage
	Vaults() ([]string, error)
}

type ItemStorage interface {
	// DeleteItem delete the item from the specified vaultName
	DeleteItem(vault *Vault, item Item) error
	// LoadItem returns a item from the vault decrypting from the underlying storage
	LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error)
	// StoreItem encrypts and encrypts and stores the item into the specified vault
	StoreItem(vault *Vault, item Item) error
}

func storageRootPath(s Storage) string {
	return filepath.Join(s.Root(), storageRootName)
}

func vaultRootPath(s Storage, name string) string {
	return filepath.Join(storageRootPath(s), name)
}

func keyPath(s Storage, name string) string {
	return filepath.Join(vaultRootPath(s, name), keyFileName)
}

func vaultPath(s Storage, name string) string {
	return filepath.Join(vaultRootPath(s, name), vaultFileName)
}

func itemPath(s Storage, vaultName string, itemID string) string {
	itemFileName := fmt.Sprintf("%s.age", itemID)
	return filepath.Join(vaultRootPath(s, vaultName), itemFileName)
}

func encrypt(key *Key, w io.Writer, v interface{}) error {
	encWriter, err := key.Encrypt(w)
	if err != nil {
		return fmt.Errorf("could not create encrypted writer for URI: %w", err)
	}
	defer encWriter.Close()

	err = json.NewEncoder(encWriter).Encode(v)
	if err != nil {
		return fmt.Errorf("could not encode data for URI: %w", err)
	}

	return nil
}

func decrypt(key *Key, r io.Reader, v interface{}) error {
	encReader, err := key.Decrypt(r)
	if err != nil {
		return fmt.Errorf("could not decrypt content: %w", err)
	}

	err = json.NewDecoder(encReader).Decode(v)
	if err != nil {
		return fmt.Errorf("could not decode content: %w", err)
	}
	return nil
}
