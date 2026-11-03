// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// Storage provides access to Paw's application state, vaults, and items.
type Storage interface {
	// Root returns the application data directory.
	Root() string
	AppStateStorage
	VaultStorage
	ItemStorage
	LogStorage
	// SocketAgentPath returns the local agent address.
	SocketAgentPath() string
	// LockFilePath returns the application lock file path.
	LockFilePath() string
}

// AppStateStorage loads and saves application settings.
type AppStateStorage interface {
	// LoadAppState reads the application settings.
	LoadAppState() (*model.AppState, error)
	// StoreAppState saves the application settings.
	StoreAppState(s *model.AppState) error
}

// VaultStorage loads and saves vaults and their encrypted keys.
type VaultStorage interface {
	// CreateVault creates an empty password-protected vault with optional public appearance metadata.
	CreateVault(name, password string, appearance ...model.VaultAppearance) (*model.Vault, error)
	// DeleteVault removes a vault by its filesystem ID.
	DeleteVault(vaultID string) error
	// LoadVault reads a vault and migrates it when it uses the legacy format.
	LoadVault(vaultID string, key *paw.Key) (*model.Vault, error)
	// LoadVaultKey decrypts a vault key by its filesystem ID.
	LoadVaultKey(vaultID string, password string) (*paw.Key, error)
	// RenameVault changes a vault's display name.
	RenameVault(vault *model.Vault, name string) error
	// UpdateVaultIdentity stores a vault's display name and public appearance.
	UpdateVaultIdentity(vault *model.Vault, name string, appearance model.VaultAppearance) error
	// ChangeMasterPassword rewraps the existing key and atomically replaces its protection file.
	ChangeMasterPassword(ctx context.Context, vaultID, currentPassword, newPassword string) error
	// Vaults lists vault IDs and display names.
	Vaults() ([]model.VaultInfo, error)
}

// ItemStorage loads and saves individual vault items.
type ItemStorage interface {
	// DeleteItem removes an item from a vault by ID.
	DeleteItem(vault *model.Vault, itemID string) error
	// LoadItem reads and decrypts an item.
	LoadItem(vault *model.Vault, itemID string) (*model.Item, error)
	// StoreItem encrypts and saves an item and its vault summary.
	StoreItem(vault *model.Vault, item *model.Item) error
}

// LogStorage provides the application log location.
type LogStorage interface {
	// LogFilePath returns the application log file path.
	LogFilePath() string
}
