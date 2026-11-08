// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.lucor.dev/paw/internal/paw"
)

// storeDocument encrypts a new document under its Item and file IDs without replacing existing files.
func storeDocument(root *os.Root, key *paw.Key, itemID, fileID string, src io.Reader) (size uint64, digest string, resultErr error) {
	if key == nil {
		return 0, "", fmt.Errorf("vault key is missing")
	}
	if !isVaultID(itemID) || !isVaultID(fileID) {
		return 0, "", fmt.Errorf("invalid document ID")
	}
	itemDirectory := filepath.Join(itemsDirectoryName, filesystemID(itemID))
	itemsInfo, err := root.Lstat(itemsDirectoryName)
	if err != nil || !itemsInfo.IsDir() || itemsInfo.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("item directory is unavailable")
	}
	itemInfo, err := root.Lstat(itemDirectory)
	if err != nil || !itemInfo.IsDir() || itemInfo.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("item directory is unavailable")
	}
	itemRoot, err := root.OpenRoot(itemDirectory)
	if err != nil {
		return 0, "", fmt.Errorf("cannot open Item directory: %w", err)
	}
	defer itemRoot.Close()

	documentsInfo, err := itemRoot.Lstat(documentsDirectoryName)
	if errors.Is(err, os.ErrNotExist) {
		if err := itemRoot.Mkdir(documentsDirectoryName, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return 0, "", fmt.Errorf("cannot create documents directory: %w", err)
		}
		documentsInfo, err = itemRoot.Lstat(documentsDirectoryName)
	}
	if err != nil || !documentsInfo.IsDir() || documentsInfo.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("documents path is unavailable")
	}
	documentsRoot, err := itemRoot.OpenRoot(documentsDirectoryName)
	if err != nil {
		return 0, "", fmt.Errorf("cannot open documents directory: %w", err)
	}
	defer documentsRoot.Close()

	finalName := documentFilename(fileID)
	if _, err := documentsRoot.Lstat(finalName); err == nil {
		return 0, "", fmt.Errorf("document already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, "", fmt.Errorf("cannot inspect document destination: %w", err)
	}

	tempName, temp, err := createDocumentTemp(documentsRoot)
	if err != nil {
		return 0, "", fmt.Errorf("cannot create encrypted document temporary: %w", err)
	}
	closed := false
	published := false
	defer func() {
		if !published {
			if !closed {
				resultErr = errors.Join(resultErr, temp.Close())
			}
			resultErr = errors.Join(resultErr, documentsRoot.Remove(tempName))
		}
	}()

	size, digest, err = encryptDocument(key, temp, src)
	if err != nil {
		return 0, "", err
	}
	if err := temp.Sync(); err != nil {
		return 0, "", fmt.Errorf("cannot sync encrypted document: %w", err)
	}
	if err := temp.Close(); err != nil {
		closed = true
		return 0, "", fmt.Errorf("cannot close encrypted document: %w", err)
	}
	closed = true
	if err := documentsRoot.Link(tempName, finalName); err != nil {
		return 0, "", fmt.Errorf("cannot publish encrypted document: %w", err)
	}
	published = true
	if err := documentsRoot.Remove(tempName); err != nil {
		return 0, "", fmt.Errorf("document published but temporary cleanup failed: %w", err)
	}
	if err := syncDirectory(documentsRoot, "."); err != nil {
		return 0, "", fmt.Errorf("document published but directory sync failed: %w", err)
	}
	if err := syncDirectory(itemRoot, "."); err != nil {
		return 0, "", fmt.Errorf("document published but Item directory sync failed: %w", err)
	}
	return size, digest, nil
}

// createDocumentTemp creates a private encrypted staging file without replacing another entry.
func createDocumentTemp(root *os.Root) (string, *os.File, error) {
	for range 10 {
		var token [12]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", nil, err
		}
		name := ".paw-document-" + hex.EncodeToString(token[:]) + ".tmp"
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
