// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage reads and writes Paw's encrypted files.
// File replacement atomicity and durability depend on the operating system and filesystem.
// Immutable Item versions are persisted before vault.age commits their references.
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"fyne.io/fyne/v2"
	"go.lucor.dev/paw/internal/paw"
)

// FileStorage stores encrypted vault data in one local application directory.
type FileStorage struct {
	root        *os.Root
	syncPeersMu sync.Mutex
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

// NewAppStorage opens Paw's configured application directory.
func NewAppStorage(app fyne.App) (*FileStorage, error) {
	root, err := appStorageRoot(app, runtime.GOOS)
	if err != nil {
		return nil, err
	}
	if root == "" {
		return NewFileStorage("")
	}
	return NewFileStorage(root)
}

func appStorageRoot(app fyne.App, goos string) (string, error) {
	if goos != "android" && goos != "ios" {
		return "", nil
	}
	if app == nil {
		return "", fmt.Errorf("mobile application storage is unavailable")
	}
	uri := app.Storage().RootURI()
	if uri == nil || uri.Scheme() != "file" || uri.Authority() != "" || uri.Query() != "" || uri.Fragment() != "" || uri.Path() == "" {
		return "", fmt.Errorf("application storage must provide a local file URI")
	}
	return uri.Path(), nil
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
	if err := (&FileStorage{root: r}).recoverIncompleteClones(); err != nil {
		_ = r.Close()
		return nil, err
	}
	return s, nil
}

// Close releases the application directory handle.
func (s *FileStorage) Close() error { return s.root.Close() }

// WriteHealthAddress stores the local service address used to detect another instance.
func (s *FileStorage) WriteHealthAddress(address string) error {
	file, err := s.root.OpenFile(lockFileName, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(address); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// ReadHealthAddress reads the local service address used to detect another instance.
func (s *FileStorage) ReadHealthAddress() (string, error) {
	file, err := s.root.Open(lockFileName)
	if err != nil {
		return "", err
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	return string(data), err
}

// Root returns the Paw data directory.
func (s *FileStorage) Root() string {
	return s.root.Name()
}
