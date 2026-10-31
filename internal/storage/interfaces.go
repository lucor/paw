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
	LoadAppState() (*paw.AppState, error)
	// StoreAppState saves the application settings.
	StoreAppState(s *paw.AppState) error
}

// VaultStorage loads and saves vaults and their encrypted keys.
type VaultStorage interface {
	// CreateVault creates an empty password-protected vault with optional public appearance metadata.
	CreateVault(name, password string, appearance ...VaultAppearance) (*model.Vault, error)
	// DeleteVault removes a vault by its filesystem ID.
	DeleteVault(vaultID string) error
	// LoadVault reads a vault and migrates it when it uses the legacy format.
	LoadVault(vaultID string, key *paw.Key) (*model.Vault, error)
	// LoadVaultKey decrypts a vault key by its filesystem ID.
	LoadVaultKey(vaultID string, password string) (*paw.Key, error)
	// RenameVault changes a vault's display name.
	RenameVault(vault *model.Vault, name string) error
	// UpdateVaultIdentity stores a vault's display name and public appearance.
	UpdateVaultIdentity(vault *model.Vault, name string, appearance VaultAppearance) error
	// ChangeMasterPassword rewraps the existing key and atomically replaces its protection file.
	ChangeMasterPassword(ctx context.Context, vaultID, currentPassword, newPassword string) error
	// Vaults lists vault IDs and display names.
	Vaults() ([]VaultInfo, error)
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

// VaultInfo contains the storage ID and public details shown to users.
type VaultInfo struct {
	// ID is the vault's canonical Base64url identifier.
	ID string
	// Name is the display name.
	Name string
	// Appearance contains the public icon and color identifiers.
	Appearance VaultAppearance
	// NeedsMigration reports whether the vault still uses the legacy format.
	NeedsMigration bool
}

// VaultAppearance contains public stable identifiers for displaying a vault.
type VaultAppearance struct {
	Icon  string
	Color string
}

// DefaultVaultAppearance returns Paw's appearance for vaults without customization.
func DefaultVaultAppearance() VaultAppearance {
	return VaultAppearance{Icon: "paw", Color: "none"}
}

// NormalizeVaultAppearance replaces unsupported identifiers with Paw defaults.
func NormalizeVaultAppearance(appearance VaultAppearance) VaultAppearance {
	if !isVaultAppearanceIcon(appearance.Icon) {
		appearance.Icon = "paw"
	}
	if !isVaultAppearanceColor(appearance.Color) {
		appearance.Color = "none"
	}
	return appearance
}

func isVaultAppearanceIcon(icon string) bool {
	switch icon {
	case "paw", "vault", "briefcase-business", "house", "archive", "shield", "folder", "key-round", "laptop":
		return true
	default:
		return false
	}
}

func isVaultAppearanceColor(color string) bool {
	switch color {
	case "none", "gray", "blue", "green", "purple", "orange", "red":
		return true
	default:
		return false
	}
}

// LogStorage provides the application log location.
type LogStorage interface {
	// LogFilePath returns the application log file path.
	LogFilePath() string
}
