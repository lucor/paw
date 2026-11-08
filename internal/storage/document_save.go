// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
)

// saveDocument verifies an encrypted document and publishes it at an explicit path without replacement.
func (s *FileStorage) saveDocument(root *os.Root, key *paw.Key, itemID, fileID, destination string, expected cxf.File) (resultErr error) {
	if key == nil {
		return errors.New("vault key is missing")
	}
	if !isVaultID(itemID) || !isVaultID(fileID) || expected.ID != fileID {
		return errors.New("invalid document ID or metadata")
	}
	if err := expected.Validate(); err != nil {
		return fmt.Errorf("invalid document metadata: %w", err)
	}
	if destination == "" {
		return errors.New("destination path is required")
	}
	absPath, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve destination path: %w", err)
	}
	base := filepath.Base(absPath)
	if base == "." || base == string(filepath.Separator) || base == ".." || base == "" {
		return errors.New("destination filename is invalid")
	}
	parent := filepath.Dir(absPath)
	if err := s.validateDestinationParent(parent); err != nil {
		return err
	}
	destinationRoot, err := os.OpenRoot(parent)
	if err != nil {
		return fmt.Errorf("open destination directory: %w", err)
	}
	defer destinationRoot.Close()
	if _, err := destinationRoot.Lstat(base); err == nil {
		return errors.New("destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect destination: %w", err)
	}

	documentRoot, err := openStoredDocumentRoot(root, itemID)
	if err != nil {
		return err
	}
	defer documentRoot.Close()
	name := documentFilename(fileID)
	info, err := documentRoot.Lstat(name)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("encrypted document is missing or not a regular file")
	}
	source, err := documentRoot.Open(name)
	if err != nil {
		return fmt.Errorf("open encrypted document: %w", err)
	}
	defer source.Close()

	tempName, temp, err := createPlainDocumentTemp(destinationRoot)
	if err != nil {
		return fmt.Errorf("create destination temporary: %w", err)
	}
	closed, published := false, false
	defer func() {
		if !published {
			if !closed {
				resultErr = errors.Join(resultErr, temp.Close())
			}
			resultErr = errors.Join(resultErr, destinationRoot.Remove(tempName))
		}
	}()
	if err := decryptDocument(key, temp, source, expected); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync verified destination: %w", err)
	}
	if err := temp.Close(); err != nil {
		closed = true
		return fmt.Errorf("close verified destination: %w", err)
	}
	closed = true
	if err := destinationRoot.Link(tempName, base); err != nil {
		return fmt.Errorf("publish verified destination: %w", err)
	}
	published = true
	if err := destinationRoot.Remove(tempName); err != nil {
		return fmt.Errorf("verified destination was published but temporary cleanup failed: %w", err)
	}
	if err := syncDirectory(destinationRoot, "."); err != nil {
		return fmt.Errorf("verified destination was published but directory sync failed: %w", err)
	}
	return nil
}

// validateDestinationParent rejects symlinks and any directory inside Paw storage.
func (s *FileStorage) validateDestinationParent(path string) error {
	storageInfo, err := s.root.Stat(storageRootName)
	if err != nil {
		return fmt.Errorf("inspect Paw storage: %w", err)
	}
	volumeRoot := filepath.VolumeName(path) + string(filepath.Separator)
	root, err := os.OpenRoot(volumeRoot)
	if err != nil {
		return fmt.Errorf("open destination volume: %w", err)
	}
	defer root.Close()
	current := "."
	for _, part := range strings.Split(strings.TrimPrefix(path, volumeRoot), string(filepath.Separator)) {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := root.Lstat(current)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("destination parent is unavailable")
		}
		if os.SameFile(info, storageInfo) {
			return errors.New("destination must be outside Paw storage")
		}
	}
	return nil
}

// openStoredDocumentRoot opens an Item's document directory after rejecting unsafe paths.
func openStoredDocumentRoot(root *os.Root, itemID string) (*os.Root, error) {
	itemPath := filepath.Join(itemsDirectoryName, filesystemID(itemID))
	for _, path := range []string{itemsDirectoryName, itemPath, filepath.Join(itemPath, documentsDirectoryName)} {
		info, err := root.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("encrypted document directory is unavailable")
		}
	}
	return root.OpenRoot(filepath.Join(itemPath, documentsDirectoryName))
}

// createPlainDocumentTemp creates a private temporary file beside the requested destination.
func createPlainDocumentTemp(root *os.Root) (string, *os.File, error) {
	for range 10 {
		var token [12]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", nil, err
		}
		name := ".paw-save-" + hex.EncodeToString(token[:]) + ".tmp"
		file, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return "", nil, err
		}
		return name, file, nil
	}
	return "", nil, os.ErrExist
}
