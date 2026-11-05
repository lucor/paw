// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage reads and writes Paw's encrypted files.
// File replacement atomicity and durability depend on the operating system and filesystem.
// Item and vault writes are sequential; a failure can leave the vault summary stale.
package storage

import (
	"fmt"
	"os"
	"path/filepath"

	"go.lucor.dev/paw/internal/paw"
)

// FileStorage stores encrypted vault data in one local application directory.
type FileStorage struct {
	root *os.Root
}

// NewFileStorage opens an application directory and keeps its root open. An empty
// root selects the desktop default under PAW_HOME or the user's home directory.
func NewFileStorage(root string) (*FileStorage, error) {
	if root != "" {
		return newFileStorage(root)
	}
	parent := os.Getenv(paw.ENV_HOME)
	if parent == "" {
		var err error
		parent, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("could not get the default root directory to use for user-specific configuration data: %w", err)
		}
	}
	root = filepath.Join(parent, ".paw")
	return newFileStorage(root)
}

// newFileStorage creates and opens the application directory.
func newFileStorage(root string) (*FileStorage, error) {
	if !filepath.IsAbs(root) {
		return nil, fmt.Errorf("storage root must be an absolute path, got %s", root)
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, fmt.Errorf("could not create storage root: %w", err)
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("could not open storage root: %w", err)
	}
	s := &FileStorage{root: r}
	if err := r.MkdirAll(storageRootName, 0o700); err != nil {
		_ = r.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the application directory handle.
func (s *FileStorage) Close() error { return s.root.Close() }

// Root returns the Paw data directory.
func (s *FileStorage) Root() string {
	return s.root.Name()
}
