// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

const (
	itemHashExtensionName = "paw.pm/item-hash"
	itemHashAlgorithm     = "sha256"
)

var errMissingStateHash = errors.New("StateHash extension is missing")

// LoadItem reads and validates an encrypted CXF item by ID.
func (s *FileStorage) LoadItem(vault *model.Vault, itemID string) (*model.Item, error) {
	summary, ok := vaultItemSummary(vault, itemID)
	if !ok {
		return nil, fmt.Errorf("item is not in the vault")
	}
	stateHash, err := summaryStateHash(summary)
	if err != nil {
		return nil, err
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return nil, fmt.Errorf("could not open item: %w", err)
	}
	defer root.Close()
	return loadItemInRoot(root, vault, itemID, stateHash)
}

// loadItemInRoot reads and decrypts one item from an open vault root.
func loadItemInRoot(root *os.Root, vault *model.Vault, itemID, stateHash string) (*model.Item, error) {
	if !vaultHasItem(vault, itemID) || !isVaultID(itemID) || !validStateHash(stateHash) {
		return nil, fmt.Errorf("item is not in the vault")
	}
	r, err := root.Open(itemRelativePath(itemID, stateHash))
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
	actualHash, err := item.StateHash()
	if err != nil || actualHash != stateHash {
		return nil, fmt.Errorf("item state hash does not match vault summary")
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

// storeItem persists an immutable version before committing its summary in the vault.
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
	stateHash, err := item.StateHash()
	if err != nil {
		return fmt.Errorf("could not calculate item state hash: %w", err)
	}
	itemDir := filepath.Join(itemsDirectoryName, filesystemID(item.ID))
	if err := root.MkdirAll(itemDir, 0o700); err != nil {
		return fmt.Errorf("could not create item directory: %w", err)
	}
	newPath := itemRelativePath(item.ID, stateHash)
	if _, err := root.Lstat(newPath); errors.Is(err, os.ErrNotExist) {
		if err := s.storeEncryptedInRoot(root, newPath, vault.Key(), item); err != nil {
			return fmt.Errorf("could not store item: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not inspect item version: %w", err)
	} else if err := verifyItemVersion(root, newPath, vault, item.ID, stateHash); err != nil {
		return fmt.Errorf("existing Item version collision: %w", err)
	}
	for _, dir := range []string{itemDir, itemsDirectoryName, "."} {
		if err := syncDirectory(root, dir); err != nil {
			return fmt.Errorf("persist Item directory: %w", err)
		}
	}
	oldHash, err := summaryStateHashForID(vault, item.ID)
	if err != nil {
		return err
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was saved: %w", err)
	}
	*vault = *updated
	if err := syncDirectory(root, "."); err != nil {
		log.Printf("storage: Vault committed but directory sync failed; preserving Item versions: %v", err)
		return nil
	}
	if oldHash != "" && oldHash != stateHash {
		s.removeItemVersionBestEffort(root, item.ID, oldHash)
	}
	s.cleanupItemVersionsBestEffort(root, vault)
	return nil
}

// deleteItem commits removal of the summary before cleaning up physical versions.
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
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was deleted: %w", err)
	}
	*vault = *updated
	if err := syncDirectory(root, "."); err != nil {
		log.Printf("storage: Vault committed but directory sync failed; preserving deleted Item files: %v", err)
		return nil
	}
	s.cleanupItemVersionsBestEffort(root, vault)
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

func vaultItemSummary(vault *model.Vault, itemID string) (cxf.Item, bool) {
	if vault == nil || len(vault.Accounts) != 1 {
		return cxf.Item{}, false
	}
	for _, item := range vault.Accounts[0].Items {
		if item.ID == itemID {
			return item, true
		}
	}
	return cxf.Item{}, false
}

func summaryStateHash(summary cxf.Item) (string, error) {
	result := ""
	for _, raw := range summary.Extensions {
		var value struct {
			Name      string `json:"name"`
			Hash      string `json:"hash"`
			Algorithm string `json:"algorithm"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		if value.Name == itemHashExtensionName {
			if result != "" || value.Algorithm != itemHashAlgorithm || !validStateHash(value.Hash) {
				return "", fmt.Errorf("item %q has an invalid or duplicate StateHash", summary.ID)
			}
			result = value.Hash
		}
	}
	if result != "" {
		return result, nil
	}
	return "", fmt.Errorf("item %q has a missing or invalid StateHash: %w", summary.ID, errMissingStateHash)
}

func validStateHash(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func summaryStateHashForID(vault *model.Vault, itemID string) (string, error) {
	summary, ok := vaultItemSummary(vault, itemID)
	if !ok {
		return "", nil
	}
	return summaryStateHash(summary)
}

func (s *FileStorage) removeItemVersionBestEffort(root *os.Root, itemID, hash string) {
	if err := root.Remove(itemRelativePath(itemID, hash)); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("storage cleanup: remove obsolete Item version %q: %v", itemID, err)
	}
}

func (s *FileStorage) cleanupItemVersionsBestEffort(root *os.Root, vault *model.Vault) {
	if err := syncDirectory(root, "."); err != nil {
		log.Printf("storage cleanup: preserve versions after directory sync failure: %v", err)
		return
	}
	dir, err := root.Open(itemsDirectoryName)
	if err != nil {
		log.Printf("storage cleanup: open Item directories: %v", err)
		return
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		log.Printf("storage cleanup: list Item directories: %v", err)
		return
	}
	for _, entry := range entries {
		itemID, ok := idFromFilesystemName(entry.Name())
		if !ok || !entry.IsDir() {
			continue
		}
		summary, indexed := vaultItemSummary(vault, itemID)
		keep := ""
		if indexed {
			keep, err = summaryStateHash(summary)
			if err != nil {
				log.Printf("storage cleanup: skip Item %q: %v", itemID, err)
				continue
			}
		}
		itemDir, openErr := root.Open(filepath.Join(itemsDirectoryName, entry.Name()))
		if openErr != nil {
			log.Printf("storage cleanup: open Item directory %q: %v", itemID, openErr)
			continue
		}
		files, readErr := itemDir.ReadDir(-1)
		_ = itemDir.Close()
		if readErr != nil {
			log.Printf("storage cleanup: list Item versions %q: %v", itemID, readErr)
			continue
		}
		for _, file := range files {
			name := file.Name()
			if !strings.HasSuffix(name, ".age") || !validStateHash(strings.TrimSuffix(name, ".age")) || file.Type() != 0 {
				continue
			}
			if name == keep+".age" {
				continue
			}
			if removeErr := root.Remove(filepath.Join(itemsDirectoryName, entry.Name(), name)); removeErr != nil {
				log.Printf("storage cleanup: remove obsolete Item version %q: %v", itemID, removeErr)
			}
		}
		if !indexed {
			// Remove only an empty directory; unknown files and documents stay intact.
			path := filepath.Join(itemsDirectoryName, entry.Name())
			dir, err := root.Open(path)
			if err != nil {
				log.Printf("storage cleanup: open directory %q: %v", itemID, err)
				continue
			}
			remaining, err := dir.ReadDir(-1)
			_ = dir.Close()
			if err != nil {
				log.Printf("storage cleanup: inspect directory %q: %v", itemID, err)
			} else if len(remaining) == 0 {
				if err := root.Remove(path); err != nil {
					log.Printf("storage cleanup: remove empty directory %q: %v", itemID, err)
				}
			}
		}
	}
}

// verifyItemVersion checks an existing immutable file before it is reused.
func verifyItemVersion(root *os.Root, name string, vault *model.Vault, id, hash string) error {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("Item version is not a regular file: %v", err)
	}
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	item, readErr := decryptItem(vault.Key(), f, id)
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	actual, err := item.StateHash()
	if err != nil || actual != hash {
		return fmt.Errorf("Item version StateHash mismatch")
	}
	return nil
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
