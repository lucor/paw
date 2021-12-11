package ui

import (
	"encoding/gob"
	"errors"
	"fmt"
	"path/filepath"
	"sort"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/storage"

	"lucor.dev/paw/internal/paw"
)

// keyring is the object that acts as layer between the UI and the storage.
type keyring struct {
	data    map[string]*paw.Vault
	storage fyne.Storage
}

func newKeyring(s fyne.Storage) (*keyring, error) {
	kr := &keyring{
		storage: s,
	}

	exists, err := storage.Exists(kr.vaultsRootURI())
	if err != nil {
		return nil, fmt.Errorf("could not check for the vaults root folder: %w", err)
	}
	if !exists {
		err = storage.CreateListable(kr.vaultsRootURI())
		if err != nil {
			return nil, fmt.Errorf("could not create the vaults root folder: %w", err)
		}
	}

	vaultsURI, err := storage.List(kr.vaultsRootURI())
	if err != nil {
		return nil, err
	}
	data := map[string]*paw.Vault{}
	for _, u := range vaultsURI {
		vaultName := u.Name()
		data[vaultName] = nil
	}
	kr.data = data
	return kr, nil
}

// CreateVault encrypts and stores an empty vault into the underlying storage.
func (kr *keyring) CreateVault(name string, secret string) (*paw.Vault, error) {
	if v := kr.data[name]; v != nil {
		return nil, errors.New("vault with the same name already exists")
	}

	key, err := paw.New(name, secret)
	if err != nil {
		return nil, err
	}

	vault := paw.NewVault(name, key)

	return vault, kr.StoreVault(vault)
}

// StoreVault encrypts and stores the vault into the underlying storage.
func (kr *keyring) StoreVault(vault *paw.Vault) error {
	vaultURI := kr.makeVaultURI(vault.Name())
	root, err := storage.Parent(vaultURI)
	if err != nil {
		return fmt.Errorf("could not retrieve vault root: %w", err)
	}

	exists, err := storage.Exists(root)
	if err != nil {
		return fmt.Errorf("could not check for vault folder: %w", err)
	}
	if !exists {
		err = storage.CreateListable(root)
		if err != nil {
			return fmt.Errorf("could not create vault folder: %w", err)
		}
	}

	vaultWriter, err := storage.Writer(vaultURI)
	if err != nil {
		return fmt.Errorf("could not create vault writer: %w", err)
	}
	defer vaultWriter.Close()

	encWriter, err := vault.Encrypt(vaultWriter)
	if err != nil {
		return fmt.Errorf("could not create encrypted vault writer: %w", err)
	}
	defer encWriter.Close()

	err = gob.NewEncoder(encWriter).Encode(vault)
	if err != nil {
		return fmt.Errorf("could not encode vault data: %w", err)
	}

	kr.data[vault.Name()] = vault
	return nil
}

// IsLockedVault returns true if the vault is locked
func (kr *keyring) IsLockedVault(name string) bool {
	v := kr.data[name]
	return v == nil
}

// LockVault locks the specified vault
func (kr *keyring) LockVault(name string) error {
	kr.data[name] = nil
	return nil
}

// UnlockVault unlocks and returns a vault decrypting from the underlying
// storage using the provided secret.
func (kr *keyring) UnlockVault(name string, secret string) (*paw.Vault, error) {
	key, err := paw.New(name, secret)
	if err != nil {
		return nil, err
	}

	vaultURI := kr.makeVaultURI(name)

	vaultReader, err := storage.Reader(vaultURI)
	if err != nil {
		return nil, fmt.Errorf("could not read vault: %w", err)
	}
	defer vaultReader.Close()
	encReader, err := key.Decrypt(vaultReader)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt vault content: %w", err)
	}

	vault := &paw.Vault{}
	err = gob.NewDecoder(encReader).Decode(vault)
	if err != nil {
		return nil, fmt.Errorf("could not decode vault content: %w", err)
	}
	vault.SetName(name)
	vault.SetKey(key)

	kr.data[name] = vault
	return vault, nil
}

// LoadVault returns the vault from the keyring, or nil if locked.
// The ok result indicates whether vault was found in the keyring.
func (kr *keyring) LoadVault(name string) (vault *paw.Vault, ok bool) {
	vault, ok = kr.data[name]
	return vault, ok
}

// Vaults returns the vaults from the keyring.
func (kr *keyring) Vaults() []string {
	list := []string{}
	for k := range kr.data {
		list = append(list, k)
	}
	sort.Strings(list)
	return list
}

func (kr *keyring) vaultsRootURI() fyne.URI {
	return storage.NewFileURI(filepath.Join(kr.storage.RootURI().Path(), "vaults"))
}

func (kr *keyring) makeVaultURI(name string) fyne.URI {
	return storage.NewFileURI(filepath.Join(kr.vaultsRootURI().Path(), name, "vault.age"))
}
