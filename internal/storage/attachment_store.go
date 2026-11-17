// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
)

// storeAttachment encrypts a new attachment under its Item and file IDs without replacing existing files.
func storeAttachment(root *os.Root, key *paw.Key, itemID, fileID string, src io.Reader) (size uint64, digest string, resultErr error) {
	return storeVerifiedAttachment(root, key, itemID, fileID, src, nil)
}

// storeVerifiedAttachment checks expected metadata before publishing an encrypted file.
func storeVerifiedAttachment(root *os.Root, key *paw.Key, itemID, fileID string, src io.Reader, expected *cxf.File) (size uint64, digest string, resultErr error) {
	if key == nil {
		return 0, "", fmt.Errorf("vault key is missing")
	}
	if !isVaultID(itemID) || !isVaultID(fileID) {
		return 0, "", fmt.Errorf("invalid attachment ID")
	}
	if expected != nil {
		if err := expected.Validate(); err != nil {
			return 0, "", fmt.Errorf("invalid attachment metadata: %w", err)
		}
		// Bound the input to the declared size so an endless stream cannot fill the disk.
		src = io.LimitReader(src, int64(expected.DecryptedSize)+1)
	}
	itemDirectory := filepath.Join(itemsDirectoryName, filesystemID(itemID))
	itemsInfo, err := root.Lstat(itemsDirectoryName)
	if errors.Is(err, os.ErrNotExist) && expected != nil {
		if err := root.Mkdir(itemsDirectoryName, 0700); err != nil {
			return 0, "", fmt.Errorf("cannot create items directory: %w", err)
		}
		itemsInfo, err = root.Lstat(itemsDirectoryName)
	}
	if err != nil || !itemsInfo.IsDir() || itemsInfo.Mode()&os.ModeSymlink != 0 {
		return 0, "", fmt.Errorf("item directory is unavailable")
	}
	itemInfo, err := root.Lstat(itemDirectory)
	if errors.Is(err, os.ErrNotExist) && expected != nil {
		if err := root.Mkdir(itemDirectory, 0700); err != nil {
			return 0, "", fmt.Errorf("cannot create item directory: %w", err)
		}
		itemInfo, err = root.Lstat(itemDirectory)
	}
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

	err = atomicWrite(filesRoot, finalName, func(writer io.Writer) error {
		size, digest, err = encryptAttachment(key, writer, src)
		if err != nil {
			return err
		}
		if expected != nil && (expected.ID != fileID || expected.DecryptedSize != size || expected.IntegrityHash != digest) {
			return fmt.Errorf("attachment content does not match metadata")
		}
		if _, err := filesRoot.Lstat(finalName); err == nil {
			return fmt.Errorf("attachment already exists")
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cannot inspect attachment destination: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, "", fmt.Errorf("cannot publish encrypted attachment: %w", err)
	}
	if err := syncDirectory(filesRoot, "."); err != nil {
		return 0, "", fmt.Errorf("attachment published but directory sync failed: %w", err)
	}
	if err := syncDirectory(itemRoot, "."); err != nil {
		return 0, "", fmt.Errorf("attachment published but Item directory sync failed: %w", err)
	}
	return size, digest, nil
}
