// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

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

	for _, item := range account.Items {
		id := filesystemID(item.ID)
		source := id + ".age"
		destination := itemRelativePath(item.ID)
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
			continue
		}
		if !sourceInfo.Mode().IsRegular() {
			return fmt.Errorf("legacy item %q is not a regular file", item.ID)
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
	}
	return nil
}
