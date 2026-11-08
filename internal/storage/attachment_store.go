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

// storeAttachment encrypts a new attachment under its Item and file IDs without replacing existing files.
func storeAttachment(root *os.Root, key *paw.Key, itemID, fileID string, src io.Reader) (size uint64, digest string, resultErr error) {
	if key == nil {
		return 0, "", fmt.Errorf("vault key is missing")
	}
	if !isVaultID(itemID) || !isVaultID(fileID) {
		return 0, "", fmt.Errorf("invalid attachment ID")
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

	filesInfo, err := itemRoot.Lstat(filesDirectoryName)
	if errors.Is(err, os.ErrNotExist) {
		if err := itemRoot.Mkdir(filesDirectoryName, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return 0, "", fmt.Errorf("cannot create files directory: %w", err)
		}
		filesInfo, err = itemRoot.Lstat(filesDirectoryName)
	}
	if err != nil || !filesInfo.IsDir() || filesInfo.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("files path is unavailable")
	}
	filesRoot, err := itemRoot.OpenRoot(filesDirectoryName)
	if err != nil {
		return 0, "", fmt.Errorf("cannot open files directory: %w", err)
	}
	defer filesRoot.Close()

	finalName := attachmentFilename(fileID)
	if _, err := filesRoot.Lstat(finalName); err == nil {
		return 0, "", fmt.Errorf("attachment already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return 0, "", fmt.Errorf("cannot inspect attachment destination: %w", err)
	}

	tempName, temp, err := createAttachmentTemp(filesRoot)
	if err != nil {
		return 0, "", fmt.Errorf("cannot create encrypted attachment temporary: %w", err)
	}
	closed := false
	published := false
	defer func() {
		if !published {
			if !closed {
				resultErr = errors.Join(resultErr, temp.Close())
			}
			resultErr = errors.Join(resultErr, filesRoot.Remove(tempName))
		}
	}()

	size, digest, err = encryptAttachment(key, temp, src)
	if err != nil {
		return 0, "", err
	}
	if err := temp.Sync(); err != nil {
		return 0, "", fmt.Errorf("cannot sync encrypted attachment: %w", err)
	}
	if err := temp.Close(); err != nil {
		closed = true
		return 0, "", fmt.Errorf("cannot close encrypted attachment: %w", err)
	}
	closed = true
	if err := filesRoot.Link(tempName, finalName); err != nil {
		return 0, "", fmt.Errorf("cannot publish encrypted attachment: %w", err)
	}
	published = true
	if err := filesRoot.Remove(tempName); err != nil {
		return 0, "", fmt.Errorf("attachment published but temporary cleanup failed: %w", err)
	}
	if err := syncDirectory(filesRoot, "."); err != nil {
		return 0, "", fmt.Errorf("attachment published but directory sync failed: %w", err)
	}
	if err := syncDirectory(itemRoot, "."); err != nil {
		return 0, "", fmt.Errorf("attachment published but Item directory sync failed: %w", err)
	}
	return size, digest, nil
}

// createAttachmentTemp creates a private encrypted staging file without replacing another entry.
func createAttachmentTemp(root *os.Root) (string, *os.File, error) {
	for range 10 {
		var token [12]byte
		if _, err := rand.Read(token[:]); err != nil {
			return "", nil, err
		}
		name := ".paw-attachment-" + hex.EncodeToString(token[:]) + ".tmp"
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
