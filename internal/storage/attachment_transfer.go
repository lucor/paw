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
	"go.lucor.dev/paw/internal/model"
)

// AttachmentReferences returns distinct current and historical attachment metadata.
func AttachmentReferences(item *model.Item) ([]cxf.File, error) {
	return itemAttachmentReferences(item)
}

// WriteAttachment decrypts and verifies an attachment into the supplied writer.
func (s *FileStorage) WriteAttachment(vault *model.Vault, itemID string, file cxf.File, dst io.Writer) error {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return err
	}
	defer root.Close()
	if !isVaultID(itemID) || !isVaultID(file.ID) {
		return fmt.Errorf("invalid attachment ID")
	}
	path := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	if _, err := root.Lstat(path); err != nil {
		return err
	}
	dir, err := openAttachmentDirectory(root, itemID)
	if err != nil {
		return err
	}
	defer dir.Close()
	name := attachmentFilename(file.ID)
	info, err := dir.Lstat(name)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("attachment is not a regular file")
	}
	source, err := dir.Open(name)
	if err != nil {
		return err
	}
	defer source.Close()
	return decryptAttachment(vault.Key(), dst, source, file)
}

// VerifyAttachment reports absence or verifies an existing blob against its metadata.
func (s *FileStorage) VerifyAttachment(vault *model.Vault, itemID string, file cxf.File) (bool, error) {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return false, err
	}
	defer root.Close()
	if !isVaultID(itemID) || !isVaultID(file.ID) {
		return false, fmt.Errorf("invalid attachment ID")
	}
	path := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	if _, err := root.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	err = s.WriteAttachment(vault, itemID, file, io.Discard)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// ReceiveAttachment publishes a verified blob with its original ID, without replacement.
// It returns true only when this call creates the blob.
func (s *FileStorage) ReceiveAttachment(vault *model.Vault, itemID string, file cxf.File, src io.Reader) (bool, error) {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return false, err
	}
	defer root.Close()
	exists, err := s.VerifyAttachment(vault, itemID, file)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}
	_, _, err = storeVerifiedAttachment(root, vault.Key(), itemID, file.ID, src, &file)
	if err != nil {
		cleanupErr := s.CleanupAttachments(vault, itemID, []string{file.ID})
		return false, errors.Join(err, cleanupErr)
	}
	return true, nil
}

// CleanupAttachments removes only supplied IDs unused by the committed item and History.
func (s *FileStorage) CleanupAttachments(vault *model.Vault, itemID string, created []string) error {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return err
	}
	defer root.Close()
	if !isVaultID(itemID) {
		return fmt.Errorf("invalid item ID")
	}
	referenced := make(map[string]bool)
	if vaultHasItem(vault, itemID) {
		item, err := s.LoadItem(vault, itemID)
		if err != nil {
			return err
		}
		files, err := itemAttachmentReferences(item)
		if err != nil {
			return err
		}
		for _, file := range files {
			referenced[file.ID] = true
		}
	}
	filesPath := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	if _, err := root.Lstat(filesPath); errors.Is(err, os.ErrNotExist) {
		_ = root.Remove(filepath.Join(itemsDirectoryName, filesystemID(itemID)))
		return nil
	} else if err != nil {
		return err
	}
	dir, err := openAttachmentDirectory(root, itemID)
	if err != nil {
		return err
	}
	defer dir.Close()
	for _, id := range created {
		if !isVaultID(id) {
			return fmt.Errorf("invalid attachment ID")
		}
		if referenced[id] {
			continue
		}
		if err := dir.Remove(attachmentFilename(id)); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
	}
	if err := syncDirectory(dir, "."); err != nil {
		return err
	}
	itemPath := filepath.Join(itemsDirectoryName, filesystemID(itemID))
	// Remove accepts empty directories only, preserving any committed item versions.
	_ = root.Remove(filepath.Join(itemPath, filesDirectoryName))
	_ = root.Remove(itemPath)
	return nil
}
