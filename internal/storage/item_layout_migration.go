// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.lucor.dev/paw/internal/model"
)

// migrateItemLayout moves legacy flat item files into their current directories.
func (s *FileStorage) migrateItemLayout(vault *model.Vault) (returnErr error) {
	account := vault.Accounts[0]
	root, err := s.openVaultRoot(account.ID)
	if err != nil {
		return fmt.Errorf("open vault directory: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			returnErr = errors.Join(returnErr, fmt.Errorf("close vault directory: %w", err))
		}
	}()
	if err := root.MkdirAll(itemsDirectoryName, 0o700); err != nil {
		return fmt.Errorf("create item storage directory: %w", err)
	}
	updated := vault
	changed := false

	for _, item := range account.Items {
		hash, hashErr := summaryStateHash(item)
		id := filesystemID(item.ID)
		source := id + ".age"
		if hashErr != nil {
			if !errors.Is(hashErr, errMissingStateHash) {
				return hashErr
			}
		}
		destination := itemRelativePath(item.ID, hash)
		sourceInfo, sourceErr := root.Lstat(source)
		destinationInfo, destinationErr := root.Lstat(destination)
		sourceExists := sourceErr == nil
		destinationExists := destinationErr == nil
		if sourceErr != nil && !errors.Is(sourceErr, os.ErrNotExist) {
			return fmt.Errorf("inspect legacy item %q: %w", item.ID, sourceErr)
		}
		if destinationErr != nil && !errors.Is(destinationErr, os.ErrNotExist) {
			return fmt.Errorf("inspect migrated item %q: %w", item.ID, destinationErr)
		}
		if sourceExists {
			if !sourceInfo.Mode().IsRegular() {
				return fmt.Errorf("legacy item %q is not a regular file", item.ID)
			}
			legacy, err := root.Open(source)
			if err != nil {
				return fmt.Errorf("open legacy Item %q: %w", item.ID, err)
			}
			migrated, readErr := decryptItem(vault.Key(), legacy, item.ID)
			closeErr := legacy.Close()
			if err := errors.Join(readErr, closeErr); err != nil {
				return fmt.Errorf("read legacy Item %q: %w", item.ID, err)
			}
			actualHash, err := migrated.StateHash()
			if err != nil {
				return fmt.Errorf("calculate StateHash for Item %q: %w", item.ID, err)
			}
			if hash != actualHash {
				updated, err = prepareItemVault(updated, migrated)
				if err != nil {
					return err
				}
				changed = true
				hash = actualHash
				destination = itemRelativePath(item.ID, hash)
				destinationInfo, destinationErr = root.Lstat(destination)
				destinationExists = destinationErr == nil
				if destinationErr != nil && !errors.Is(destinationErr, os.ErrNotExist) {
					return fmt.Errorf("inspect recalculated version for Item %q: %w", item.ID, destinationErr)
				}
			}
		}
		if sourceExists && destinationExists {
			return fmt.Errorf("item %q exists in both legacy and current storage paths", item.ID)
		}
		if destinationExists {
			if !destinationInfo.Mode().IsRegular() {
				return fmt.Errorf("migrated item %q is not a regular file", item.ID)
			}
			continue
		}
		if !sourceExists {
			// A rename may have completed before the Vault summary was
			// committed. Recover only a single, verifiable hashed version.
			recoveredHash, found, err := recoverMigratedItem(root, vault, item.ID)
			if err != nil {
				return err
			}
			if found {
				updated, err = prepareItemVault(updated, recoveredHash.item)
				if err != nil {
					return err
				}
				changed = true
				// The interrupted rename may not have reached disk yet.
				for _, dir := range []string{filepath.Dir(itemRelativePath(item.ID, recoveredHash.hash)), itemsDirectoryName, "."} {
					if err := syncDirectory(root, dir); err != nil {
						return fmt.Errorf("persist recovered Item %q: %w", item.ID, err)
					}
				}
				continue
			}
		}
		if !sourceExists && !destinationExists {
			if hashErr != nil {
				return hashErr
			}
			return fmt.Errorf("item %q has no committed storage file", item.ID)
		}
		itemDir := filepath.Dir(destination)
		if err := root.MkdirAll(itemDir, 0o700); err != nil {
			return fmt.Errorf("create storage directory for item %q: %w", item.ID, err)
		}
		// Paw uses storage exclusively; the destination checked above cannot be
		// created by another writer before this same-filesystem rename.
		if err := root.Rename(source, destination); err != nil {
			return fmt.Errorf("move item %q into current storage: %w", item.ID, err)
		}
		for _, dir := range []string{itemDir, itemsDirectoryName, "."} {
			if err := syncDirectory(root, dir); err != nil {
				return err
			}
		}
	}
	if changed {
		updated.Timestamp = uint64(time.Now().Unix())
		if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
			return fmt.Errorf("commit migrated Item StateHashes: %w", err)
		}
		if err := syncDirectory(root, "."); err != nil {
			return fmt.Errorf("persist migrated Vault: %w", err)
		}
		*vault = *updated
	}
	return nil
}

type recoveredItemVersion struct {
	item *model.Item
	hash string
}

// recoverMigratedItem finds a single hashed version left by an interrupted migration.
func recoverMigratedItem(root *os.Root, vault *model.Vault, itemID string) (recoveredItemVersion, bool, error) {
	itemDir := filepath.Join(itemsDirectoryName, filesystemID(itemID))
	dir, err := root.Open(itemDir)
	if errors.Is(err, os.ErrNotExist) {
		return recoveredItemVersion{}, false, nil
	}
	if err != nil {
		return recoveredItemVersion{}, false, fmt.Errorf("open storage directory for Item %q: %w", itemID, err)
	}
	entries, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return recoveredItemVersion{}, false, fmt.Errorf("list storage directory for Item %q: %w", itemID, err)
	}
	var candidate string
	for _, entry := range entries {
		name := entry.Name()
		if filepath.Ext(name) != ".age" || !validStateHash(strings.TrimSuffix(name, ".age")) {
			continue
		}
		if !entry.Type().IsRegular() {
			return recoveredItemVersion{}, false, fmt.Errorf("hashed version for Item %q is not a regular file", itemID)
		}
		if candidate != "" {
			return recoveredItemVersion{}, false, fmt.Errorf("multiple uncommitted versions found for Item %q", itemID)
		}
		candidate = filepath.Join(itemDir, name)
	}
	if candidate == "" {
		return recoveredItemVersion{}, false, nil
	}
	file, err := root.Open(candidate)
	if err != nil {
		return recoveredItemVersion{}, false, fmt.Errorf("open interrupted Item version %q: %w", itemID, err)
	}
	item, readErr := decryptItem(vault.Key(), file, itemID)
	closeErr = file.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return recoveredItemVersion{}, false, fmt.Errorf("read interrupted Item version %q: %w", itemID, err)
	}
	hash, err := item.StateHash()
	if err != nil {
		return recoveredItemVersion{}, false, fmt.Errorf("calculate StateHash for interrupted Item %q: %w", itemID, err)
	}
	if filepath.Base(candidate) != hash+".age" {
		return recoveredItemVersion{}, false, fmt.Errorf("interrupted Item %q has a StateHash that does not match its filename", itemID)
	}
	return recoveredItemVersion{item: item, hash: hash}, true, nil
}
