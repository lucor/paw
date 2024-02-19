package paw

import (
	"errors"
	"os"
	"path/filepath"
)

var _ Storage = (*StorageMock)(nil)
var _ ConfigStorage = (*ConfigStorageMock)(nil)
var _ ItemStorage = (*ItemStorageMock)(nil)
var _ VaultStorage = (*VaultStorageMock)(nil)

var (
	ErrCallbackRequired = errors.New("callback required")
)

type StorageMock struct {
	ConfigStorageMock
	VaultStorageMock
	ItemStorageMock
	OnSocketAgentPath func() string
}

// LockFilePath implements Storage.
func (*StorageMock) LockFilePath() string {
	return filepath.Join(os.TempDir(), "paw_lock_file_mock")
}

// LogFilePath implements Storage.
func (*StorageMock) LogFilePath() string {
	return filepath.Join(os.TempDir(), "paw_log_file_mock")
}

// Root implements Storage.
func (c *StorageMock) Root() string {
	return filepath.Join(os.TempDir(), "paw_root_mock")
}

// SocketAgentPath implements Storage.
func (c *StorageMock) SocketAgentPath() string {
	return filepath.Join(os.TempDir(), "paw_socket_agent_path_mock")
}

type ConfigStorageMock struct {
	OnLoadConfig  func() (*Config, error)
	OnStoreConfig func(s *Config) error
}

// LoadConfig implements ConfigStorage.
func (c *ConfigStorageMock) LoadConfig() (*Config, error) {
	if c.OnLoadConfig == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadConfig()
}

// StoreConfig implements ConfigStorage.
func (c *ConfigStorageMock) StoreConfig(s *Config) error {
	if c.OnStoreConfig == nil {
		return ErrCallbackRequired
	}
	return c.OnStoreConfig(s)
}

type VaultStorageMock struct {
	// CreateVault encrypts and stores an empty vault into the underlying storage.
	OnCreateVault func(name string, key *Key) (*Vault, error)
	// LoadVaultKey creates and stores a Key used to encrypt and decrypt the vault data
	// The file containing the key is encrypted using the provided password
	OnCreateVaultKey func(name string, password string) (*Key, error)
	// DeleteVault delete the specified vault
	OnDeleteVault func(name string) error
	// LoadVault returns a vault decrypting from the underlying storage
	OnLoadVault func(name string, key *Key) (*Vault, error)
	// LoadVaultKey returns the Key used to encrypt and decrypt the vault data
	OnLoadVaultKey func(name string, password string) (*Key, error)
	// StoreVault encrypts and stores the vault into the underlying storage
	OnStoreVault func(vault *Vault) error
	// Vaults returns the list of vault names from the storage
	OnVaults func() ([]string, error)
}

// CreateVault implements VaultStorage.
func (c *VaultStorageMock) CreateVault(name string, key *Key) (*Vault, error) {
	if c.OnCreateVault == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnCreateVault(name, key)
}

// CreateVaultKey implements VaultStorage.
func (c *VaultStorageMock) CreateVaultKey(name string, password string) (*Key, error) {
	if c.OnCreateVaultKey == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnCreateVaultKey(name, password)
}

// DeleteVault implements VaultStorage.
func (c *VaultStorageMock) DeleteVault(name string) error {
	if c.OnDeleteVault == nil {
		return nil
	}
	return c.OnDeleteVault(name)
}

// LoadVault implements VaultStorage.
func (c *VaultStorageMock) LoadVault(name string, key *Key) (*Vault, error) {
	if c.OnLoadVault == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadVault(name, key)
}

// LoadVaultKey implements VaultStorage.
func (c *VaultStorageMock) LoadVaultKey(name string, password string) (*Key, error) {
	if c.OnLoadVaultKey == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadVaultKey(name, password)
}

// StoreVault implements VaultStorage.
func (c *VaultStorageMock) StoreVault(vault *Vault) error {
	if c.OnStoreVault == nil {
		return ErrCallbackRequired
	}
	return c.OnStoreVault(vault)
}

// Vaults implements VaultStorage.
func (c *VaultStorageMock) Vaults() ([]string, error) {
	if c.OnVaults == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnVaults()
}

type ItemStorageMock struct {
	// DeleteItem delete the item from the specified vaultName
	OnDeleteItem func(vault *Vault, item Item) error
	// LoadItem returns a item from the vault decrypting from the underlying storage
	OnLoadItem func(vault *Vault, itemMetadata *Metadata) (Item, error)
	// StoreItem encrypts and encrypts and stores the item into the specified vault
	OnStoreItem func(vault *Vault, item Item) error
}

// DeleteItem implements ItemStorage.
func (c *ItemStorageMock) DeleteItem(vault *Vault, item Item) error {
	if c.OnDeleteItem == nil {
		return ErrCallbackRequired
	}
	return c.OnDeleteItem(vault, item)
}

// LoadItem implements ItemStorage.
func (c *ItemStorageMock) LoadItem(vault *Vault, itemMetadata *Metadata) (Item, error) {
	if c.OnLoadItem == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadItem(vault, itemMetadata)
}

// StoreItem implements ItemStorage.
func (c *ItemStorageMock) StoreItem(vault *Vault, item Item) error {
	if c.OnStoreItem == nil {
		return ErrCallbackRequired
	}
	return c.OnStoreItem(vault, item)
}
