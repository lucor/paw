// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"fmt"

	fynestorage "fyne.io/fyne/v2/storage"

	"go.lucor.dev/paw/internal/legacy"
	"go.lucor.dev/paw/internal/paw"
)

// loadLegacyVault reads a vault using the legacy Paw format.
func (s *FyneStorage) loadLegacyVault(vaultID string, key *paw.Key) (*legacy.Vault, error) {
	data, err := s.readLegacyVaultData(vaultID, key)
	if err != nil {
		return nil, err
	}
	return legacy.DecodeVault(data, key)
}

// readLegacyVaultData reads and decrypts the index at its legacy path.
func (s *FyneStorage) readLegacyVaultData(vaultID string, key *paw.Key) ([]byte, error) {
	r, err := fynestorage.Reader(fynestorage.NewFileURI(legacyVaultPath(s, vaultID)))
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the vault: %w", err)
	}
	return data, nil
}

// loadLegacyItem reads an item using the legacy Paw format.
func (s *FyneStorage) loadLegacyItem(vault *legacy.Vault, itemMetadata *legacy.Metadata) (legacy.Item, error) {
	r, err := fynestorage.Reader(fynestorage.NewFileURI(legacyItemPath(s, vault.Name, itemMetadata.ID())))
	if err != nil {
		return nil, fmt.Errorf("could not create reader: %w", err)
	}
	defer r.Close()

	data, err := readDecrypted(vault.Key(), r)
	if err != nil {
		return nil, fmt.Errorf("could not read and decrypt the item: %w", err)
	}
	return legacy.DecodeItem(data, itemMetadata.Type)
}
