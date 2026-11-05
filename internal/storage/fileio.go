// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.lucor.dev/paw/internal/paw"
)

// readDir returns a directory's entries sorted by name.
func (s *FileStorage) readDir(name string) ([]os.DirEntry, error) {
	dir, err := s.root.Open(name)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, nil
}

// openVaultRoot opens the root for a vault with a canonical ID.
func (s *FileStorage) openVaultRoot(vaultID string) (*os.Root, error) {
	if !isVaultID(vaultID) {
		return nil, fmt.Errorf("invalid vault ID")
	}
	return s.openNamedVaultRoot(filesystemID(vaultID))
}

// openNamedVaultRoot opens a vault directory after rejecting symlink paths.
func (s *FileStorage) openNamedVaultRoot(name string) (*os.Root, error) {
	storageInfo, err := s.root.Lstat(storageRootName)
	if err != nil {
		return nil, err
	}
	if !storageInfo.IsDir() || storageInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("storage directory is not a local directory")
	}
	path := filepath.Join(storageRootName, name)
	info, err := s.root.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("vault path is not a local directory")
	}
	return s.root.OpenRoot(path)
}

// readEncryptedRoot reads the complete authenticated payload from an open vault root.
func readEncryptedRoot(root *os.Root, name string, key *paw.Key) ([]byte, error) {
	r, err := root.Open(name)
	if err != nil {
		return nil, fmt.Errorf("could not open vault: %w", err)
	}
	defer r.Close()
	data, err := readDecrypted(key, r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt vault: %w", err)
	}
	return data, nil
}

// storeEncryptedInRoot encrypts a value and atomically replaces its file.
func (s *FileStorage) storeEncryptedInRoot(root *os.Root, name string, key *paw.Key, value any) error {
	return atomicWrite(root, name, func(w io.Writer) error { return encrypt(key, w, value) })
}
