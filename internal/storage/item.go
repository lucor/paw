// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	json "encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

const (
	itemHashExtensionName = "paw.pm/item-hash"
	itemHashAlgorithm     = "sha256"
)

// LoadItem reads and validates an encrypted CXF item by ID.
func (s *FileStorage) LoadItem(vault *model.Vault, itemID string) (*model.Item, error) {
	if !vaultHasItem(vault, itemID) {
		return nil, fmt.Errorf("item is not in the vault")
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return nil, fmt.Errorf("could not open item: %w", err)
	}
	defer root.Close()
	return loadItemInRoot(root, vault, itemID)
}

// loadItemInRoot reads and decrypts one item from an open vault root.
func loadItemInRoot(root *os.Root, vault *model.Vault, itemID string) (*model.Item, error) {
	if !vaultHasItem(vault, itemID) || !isVaultID(itemID) {
		return nil, fmt.Errorf("item is not in the vault")
	}
	r, err := root.Open(filesystemID(itemID) + ".age")
	if err != nil {
		return nil, fmt.Errorf("could not open item: %w", err)
	}
	defer r.Close()
	data, err := readDecrypted(vault.Key(), r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt item: %w", err)
	}
	item, err := decodeModelItem(data)
	if err != nil {
		return nil, err
	}
	if item.ID != itemID {
		return nil, fmt.Errorf("item ID does not match requested ID")
	}
	return item, nil
}

// StoreItem saves a CXF item and updates its vault summary.
func (s *FileStorage) StoreItem(vault *model.Vault, item *model.Item) error {
	return s.storeItem(vault, item)
}

// RestoreItem restores a history entry if it still matches the caller's selection.
// It returns false when the selected version has no meaningful differences.
// now is the UTC Unix timestamp used for capture and modification times.
func (s *FileStorage) RestoreItem(
	vault *model.Vault,
	itemID string,
	index int,
	expected *model.HistoryEntry,
	now uint64,
) (*model.Item, bool, error) {
	if vault == nil {
		return nil, false, fmt.Errorf("cannot restore item: vault is required")
	}
	if err := vault.Validate(); err != nil {
		return nil, false, fmt.Errorf("cannot restore item: invalid vault: %w", err)
	}
	if !vaultHasItem(vault, itemID) {
		return nil, false, fmt.Errorf("cannot restore item: item is not in the vault")
	}
	current, err := s.LoadItem(vault, itemID)
	if err != nil {
		return nil, false, fmt.Errorf("cannot load current item for restore: %w", err)
	}
	restored, changed, err := model.PrepareItemRestore(current, index, expected, now)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return current, false, nil
	}
	if err := s.StoreItem(vault, restored); err != nil {
		return nil, false, fmt.Errorf("cannot persist restored item: %w", err)
	}
	return restored, true, nil
}

// DeleteItem removes a CXF item and its vault summary.
func (s *FileStorage) DeleteItem(vault *model.Vault, itemID string) error {
	return s.deleteItem(vault, itemID)
}

// storeItem saves the full item before saving its summary and advancing the vault.
// The two replacements are sequential, so a vault save failure can leave its summary stale.
func (s *FileStorage) storeItem(vault *model.Vault, item *model.Item) error {
	if err := vault.Validate(); err != nil {
		return fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return fmt.Errorf("could not store item: vault key is missing")
	}
	if err := item.Validate(); err != nil {
		return fmt.Errorf("could not prepare item: %w", err)
	}
	id := vault.Accounts[0].ID
	root, err := s.openVaultRoot(id)
	if err != nil {
		return fmt.Errorf("could not open vault: %w", err)
	}
	defer root.Close()

	updated, err := prepareItemVault(vault, item)
	if err != nil {
		return err
	}
	if err := s.storeEncryptedInRoot(root, filepath.Base(itemPath(s, id, item.ID)), vault.Key(), item); err != nil {
		return fmt.Errorf("could not store item: %w", err)
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was saved: %w", err)
	}
	*vault = *updated
	return nil
}

// deleteItem removes the encrypted item before saving the updated vault summary.
func (s *FileStorage) deleteItem(vault *model.Vault, itemID string) error {
	updated, err := prepareItemDeleteVault(vault, itemID)
	if err != nil {
		return err
	}
	id := updated.Accounts[0].ID
	root, err := s.openVaultRoot(id)
	if err != nil {
		return fmt.Errorf("could not open vault: %w", err)
	}
	defer root.Close()
	if err := root.Remove(filepath.Base(itemPath(s, id, itemID))); err != nil {
		return fmt.Errorf("could not delete item file: %w", err)
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was deleted: %w", err)
	}
	*vault = *updated
	return nil
}

// vaultHasItem checks whether an item summary belongs to the vault.
func vaultHasItem(vault *model.Vault, itemID string) bool {
	if vault == nil || len(vault.Accounts) != 1 {
		return false
	}
	for _, item := range vault.Accounts[0].Items {
		if item.ID == itemID {
			return true
		}
	}
	return false
}

// prepareItemVault returns a separate vault with the item's derived summary.
func prepareItemVault(vault *model.Vault, item *model.Item) (*model.Vault, error) {
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("could not store item: vault key is missing")
	}
	summary, err := item.Summary()
	if err != nil {
		return nil, fmt.Errorf("could not prepare item summary: %w", err)
	}
	stateHash, err := item.StateHash()
	if err != nil {
		return nil, fmt.Errorf("could not calculate item state hash: %w", err)
	}
	hashExtension, err := json.Marshal(struct {
		Name      string `json:"name"`
		Algorithm string `json:"algorithm"`
		Hash      string `json:"hash"`
	}{
		Name:      itemHashExtensionName,
		Algorithm: itemHashAlgorithm,
		Hash:      stateHash,
	}, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("could not encode item state hash: %w", err)
	}
	summary.Extensions = append(summary.Extensions, hashExtension)

	updated := *vault
	updated.Accounts = slices.Clone(vault.Accounts)
	account := &updated.Accounts[0]
	account.Items = slices.Clone(account.Items)
	// Vault marshaling replaces its Paw extension, so isolate that slice too.
	account.Extensions = slices.Clone(account.Extensions)
	index := slices.IndexFunc(account.Items, func(candidate cxf.Item) bool {
		return candidate.ID == item.ID
	})
	if index < 0 {
		account.Items = append(account.Items, *summary)
	} else {
		account.Items[index] = *summary
	}
	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare updated vault: %w", err)
	}
	return &updated, nil
}

// prepareItemDeleteVault returns a separate vault without the selected item summary.
func prepareItemDeleteVault(vault *model.Vault, itemID string) (*model.Vault, error) {
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("could not delete item: vault key is missing")
	}

	updated := *vault
	updated.Accounts = slices.Clone(vault.Accounts)
	account := &updated.Accounts[0]
	account.Items = slices.Clone(account.Items)
	account.Extensions = slices.Clone(account.Extensions)
	index := slices.IndexFunc(account.Items, func(candidate cxf.Item) bool {
		return candidate.ID == itemID
	})
	if index < 0 {
		return nil, fmt.Errorf("could not delete item: item %q is not in the vault", itemID)
	}
	account.Items = slices.Delete(account.Items, index, index+1)
	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare updated vault: %w", err)
	}
	return &updated, nil
}
