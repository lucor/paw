// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"errors"
	"os"
	"path/filepath"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

var _ Storage = (*StorageMock)(nil)
var _ AppStateStorage = (*AppStateStorageMock)(nil)
var _ ItemStorage = (*ItemStorageMock)(nil)
var _ VaultStorage = (*VaultStorageMock)(nil)

// ErrCallbackRequired reports that a mock method has no configured callback.
var ErrCallbackRequired = errors.New("callback required")

// StorageMock implements Storage through callbacks.
type StorageMock struct {
	AppStateStorageMock
	VaultStorageMock
	ItemStorageMock
	OnSocketAgentPath func() string
}

// LockFilePath returns a stable temporary path for tests.
func (*StorageMock) LockFilePath() string {
	return filepath.Join(os.TempDir(), "paw_lock_file_mock")
}

// LogFilePath returns a stable temporary path for tests.
func (*StorageMock) LogFilePath() string {
	return filepath.Join(os.TempDir(), "paw_log_file_mock")
}

// Root returns a stable temporary path for tests.
func (*StorageMock) Root() string {
	return filepath.Join(os.TempDir(), "paw_root_mock")
}

// SocketAgentPath returns a stable temporary path for tests.
func (*StorageMock) SocketAgentPath() string {
	return filepath.Join(os.TempDir(), "paw_socket_agent_path_mock")
}

// AppStateStorageMock implements AppStateStorage through callbacks.
type AppStateStorageMock struct {
	OnLoadAppState  func() (*paw.AppState, error)
	OnStoreAppState func(s *paw.AppState) error
}

// LoadAppState loads app state through the configured callback.
func (c *AppStateStorageMock) LoadAppState() (*paw.AppState, error) {
	if c.OnLoadAppState == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadAppState()
}

// StoreAppState stores app state through the configured callback.
func (c *AppStateStorageMock) StoreAppState(s *paw.AppState) error {
	if c.OnStoreAppState == nil {
		return ErrCallbackRequired
	}
	return c.OnStoreAppState(s)
}

// VaultStorageMock implements VaultStorage through callbacks.
type VaultStorageMock struct {
	OnCreateVault  func(name, password string) (*model.Vault, error)
	OnDeleteVault  func(vaultID string) error
	OnLoadVault    func(vaultID string, key *paw.Key) (*model.Vault, error)
	OnLoadVaultKey func(vaultID string, password string) (*paw.Key, error)
	OnRenameVault  func(vault *model.Vault, name string) error
	OnVaults       func() ([]VaultInfo, error)
}

// CreateVault creates a vault through the configured callback.
func (c *VaultStorageMock) CreateVault(name, password string) (*model.Vault, error) {
	if c.OnCreateVault == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnCreateVault(name, password)
}

// DeleteVault deletes a vault through the configured callback.
func (c *VaultStorageMock) DeleteVault(vaultID string) error {
	if c.OnDeleteVault == nil {
		return nil
	}
	return c.OnDeleteVault(vaultID)
}

// LoadVault loads a vault through the configured callback.
func (c *VaultStorageMock) LoadVault(vaultID string, key *paw.Key) (*model.Vault, error) {
	if c.OnLoadVault == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadVault(vaultID, key)
}

// LoadVaultKey loads a vault key through the configured callback.
func (c *VaultStorageMock) LoadVaultKey(name string, password string) (*paw.Key, error) {
	if c.OnLoadVaultKey == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadVaultKey(name, password)
}

// RenameVault updates a vault through the configured callback.
func (c *VaultStorageMock) RenameVault(vault *model.Vault, name string) error {
	if c.OnRenameVault == nil {
		return ErrCallbackRequired
	}
	return c.OnRenameVault(vault, name)
}

// Vaults lists vaults through the configured callback.
func (c *VaultStorageMock) Vaults() ([]VaultInfo, error) {
	if c.OnVaults == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnVaults()
}

// ItemStorageMock implements ItemStorage through callbacks.
type ItemStorageMock struct {
	OnDeleteItem func(vault *model.Vault, itemID string) error
	OnLoadItem   func(vault *model.Vault, itemID string) (*model.Item, error)
	OnStoreItem  func(vault *model.Vault, item *model.Item) error
}

// DeleteItem deletes an item through the configured callback.
func (c *ItemStorageMock) DeleteItem(vault *model.Vault, itemID string) error {
	if c.OnDeleteItem == nil {
		return ErrCallbackRequired
	}
	return c.OnDeleteItem(vault, itemID)
}

// LoadItem loads an item through the configured callback.
func (c *ItemStorageMock) LoadItem(vault *model.Vault, itemID string) (*model.Item, error) {
	if c.OnLoadItem == nil {
		return nil, ErrCallbackRequired
	}
	return c.OnLoadItem(vault, itemID)
}

// StoreItem stores an item through the configured callback.
func (c *ItemStorageMock) StoreItem(vault *model.Vault, item *model.Item) error {
	if c.OnStoreItem == nil {
		return ErrCallbackRequired
	}
	return c.OnStoreItem(vault, item)
}
