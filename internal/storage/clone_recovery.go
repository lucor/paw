// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// CompleteClone removes the recovery marker after a clone has fully transferred.
func (s *FileStorage) CompleteClone(vaultID string) error {
	root, err := s.openVaultRoot(vaultID)
	if err != nil {
		return fmt.Errorf("could not open cloned vault: %w", err)
	}
	defer root.Close()
	if err := root.Remove(clonePendingFileName); err != nil {
		return fmt.Errorf("could not mark cloned vault complete: %w", err)
	}
	return nil
}

// recoverIncompleteClones removes vaults left behind by an interrupted clone.
func (s *FileStorage) recoverIncompleteClones() error {
	entries, err := s.readDir(storageRootName)
	if err != nil {
		return fmt.Errorf("could not inspect stored vaults for incomplete clones: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() || !isFilesystemVaultID(entry.Name()) {
			continue
		}
		vaultPath := filepath.Join(storageRootName, entry.Name())
		vault, err := s.root.OpenRoot(vaultPath)
		if err != nil {
			return fmt.Errorf("could not inspect vault for incomplete clone: %w", err)
		}
		_, markerErr := vault.Lstat(clonePendingFileName)
		closeErr := vault.Close()
		if markerErr != nil && !os.IsNotExist(markerErr) {
			return fmt.Errorf("could not inspect incomplete clone marker: %w", markerErr)
		}
		if err := closeErr; err != nil {
			return fmt.Errorf("could not close vault after clone recovery: %w", err)
		}
		if markerErr == nil {
			if err := s.root.RemoveAll(vaultPath); err != nil {
				return fmt.Errorf("could not remove incomplete clone: %w", err)
			}
		}
	}
	return nil
}
