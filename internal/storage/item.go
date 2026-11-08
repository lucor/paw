// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// AddAttachment encrypts and commits one attachment through the normal Item history flow.
func (s *FileStorage) AddAttachment(vault *model.Vault, itemID, name string, src io.Reader) (cxf.File, error) {
	if vault == nil || vault.Key() == nil {
		return cxf.File{}, fmt.Errorf("cannot add attachment: unlocked vault is required")
	}
	current, err := s.LoadItem(vault, itemID)
	if err != nil {
		return cxf.File{}, fmt.Errorf("cannot load Item: %w", err)
	}
	if current.Metadata.Type != model.NoteItemType && current.Metadata.Type != model.LoginItemType {
		return cxf.File{}, fmt.Errorf("attachments are not supported for this Item type")
	}
	fileID, err := model.NewFileID()
	if err != nil {
		return cxf.File{}, err
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return cxf.File{}, err
	}
	defer root.Close()
	size, digest, err := storeAttachment(root, vault.Key(), itemID, fileID, src)
	if err != nil {
		return cxf.File{}, err
	}
	file := cxf.File{Type: cxf.CredentialTypeFile, ID: fileID, Name: name, DecryptedSize: size, IntegrityHash: digest}
	proposed := *current
	proposed.Credentials = slices.Clone(current.Credentials)
	if err := proposed.AddFile(file); err != nil {
		return cxf.File{}, err
	}
	now := uint64(time.Now().Unix())
	updated, changed, err := model.PrepareItemEdit(current, &proposed, now, now)
	if err != nil {
		return cxf.File{}, err
	}
	if !changed {
		return cxf.File{}, fmt.Errorf("attachment did not change Item")
	}
	if err := s.StoreItem(vault, updated); err != nil {
		return cxf.File{}, fmt.Errorf("cannot commit attachment: %w", err)
	}
	return file, nil
}

// ListAttachments returns current CXF file metadata without opening attachment blobs.
func (s *FileStorage) ListAttachments(vault *model.Vault, itemID string) ([]cxf.File, error) {
	if vault == nil {
		return nil, fmt.Errorf("cannot list attachments: vault is required")
	}
	item, err := s.LoadItem(vault, itemID)
	if err != nil {
		return nil, fmt.Errorf("cannot load Item: %w", err)
	}
	files, err := item.Files()
	if err != nil {
		return nil, fmt.Errorf("cannot list attachment metadata: %w", err)
	}
	return files, nil
}

// SaveAttachment decrypts one current attachment to an explicit destination without changing the vault.
func (s *FileStorage) SaveAttachment(vault *model.Vault, itemID, fileID, destination string) error {
	if vault == nil || vault.Key() == nil {
		return fmt.Errorf("cannot save attachment: unlocked vault is required")
	}
	item, err := s.LoadItem(vault, itemID)
	if err != nil {
		return fmt.Errorf("cannot load Item: %w", err)
	}
	files, err := item.Files()
	if err != nil {
		return fmt.Errorf("cannot read attachment metadata: %w", err)
	}
	var selected *cxf.File
	for i := range files {
		if files[i].ID == fileID {
			selected = &files[i]
			break
		}
	}
	if selected == nil {
		return fmt.Errorf("attachment ID does not exist")
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return err
	}
	defer root.Close()
	return s.saveAttachmentFile(root, vault.Key(), itemID, fileID, destination, *selected)
}

// RemoveAttachment removes one file credential and records the change in Item history.
func (s *FileStorage) RemoveAttachment(vault *model.Vault, itemID, fileID string) error {
	if vault == nil || vault.Key() == nil {
		return fmt.Errorf("cannot remove attachment: unlocked vault is required")
	}
	current, err := s.LoadItem(vault, itemID)
	if err != nil {
		return fmt.Errorf("cannot load Item: %w", err)
	}
	proposed := *current
	proposed.Credentials = slices.Clone(current.Credentials)
	if err := proposed.RemoveFile(fileID); err != nil {
		return err
	}
	now := uint64(time.Now().Unix())
	updated, changed, err := model.PrepareItemEdit(current, &proposed, now, now)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	return s.StoreItem(vault, updated)
}

const (
	itemHashExtensionName = "paw.pm/item-hash"
	itemHashAlgorithm     = "sha256"
)

var errMissingStateHash = errors.New("StateHash extension is missing")

// LoadItem reads and validates an encrypted CXF item by ID.
func (s *FileStorage) LoadItem(vault *model.Vault, itemID string) (*model.Item, error) {
	summary, ok := vaultItemSummary(vault, itemID)
	if !ok {
		return nil, fmt.Errorf("item is not in the vault")
	}
	stateHash, err := summaryStateHash(summary)
	if err != nil {
		return nil, err
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return nil, fmt.Errorf("could not open item: %w", err)
	}
	defer root.Close()
	return loadItemInRoot(root, vault, itemID, stateHash)
}

// loadItemInRoot reads and decrypts one item from an open vault root.
func loadItemInRoot(root *os.Root, vault *model.Vault, itemID, stateHash string) (*model.Item, error) {
	if !vaultHasItem(vault, itemID) || !isVaultID(itemID) || !validStateHash(stateHash) {
		return nil, fmt.Errorf("item is not in the vault")
	}
	r, err := root.Open(itemRelativePath(itemID, stateHash))
	if err != nil {
		return nil, fmt.Errorf("could not open item: %w", err)
	}
	defer r.Close()
	data, err := readDecrypted(vault.Key(), r)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt item: %w", err)
	}
	item, err := decodeModelItem(data)
	if err != nil {
		return nil, err
	}
	if item.ID != itemID {
		return nil, fmt.Errorf("item ID does not match requested ID")
	}
	actualHash, err := item.StateHash()
	if err != nil || actualHash != stateHash {
		return nil, fmt.Errorf("item state hash does not match vault summary")
	}
	return item, nil
}

// StoreItem saves a CXF item and updates its vault summary.
func (s *FileStorage) StoreItem(vault *model.Vault, item *model.Item) error {
	return s.storeItem(vault, item)
}

// RestoreItem restores a history entry if it still matches the caller's selection.
// It returns false when the selected version has no meaningful differences.
// now is the UTC Unix timestamp used for capture and modification times.
func (s *FileStorage) RestoreItem(
	vault *model.Vault,
	itemID string,
	index int,
	expected *model.HistoryEntry,
	now uint64,
) (*model.Item, bool, error) {
	if vault == nil {
		return nil, false, fmt.Errorf("cannot restore item: vault is required")
	}
	if err := vault.Validate(); err != nil {
		return nil, false, fmt.Errorf("cannot restore item: invalid vault: %w", err)
	}
	if !vaultHasItem(vault, itemID) {
		return nil, false, fmt.Errorf("cannot restore item: item is not in the vault")
	}
	current, err := s.LoadItem(vault, itemID)
	if err != nil {
		return nil, false, fmt.Errorf("cannot load current item for restore: %w", err)
	}
	restored, changed, err := model.PrepareItemRestore(current, index, expected, now)
	if err != nil {
		return nil, false, err
	}
	if !changed {
		return current, false, nil
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return nil, false, err
	}
	err = verifyItemAttachments(root, vault.Key(), restored)
	_ = root.Close()
	if err != nil {
		return nil, false, fmt.Errorf("cannot restore Item with unavailable attachments: %w", err)
	}
	if err := s.StoreItem(vault, restored); err != nil {
		return nil, false, fmt.Errorf("cannot persist restored item: %w", err)
	}
	return restored, true, nil
}

// verifyItemAttachments authenticates every blob referenced by an Item before restoring it.
func verifyItemAttachments(root *os.Root, key *paw.Key, item *model.Item) error {
	files, err := item.Files()
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return nil
	}
	dir, err := openAttachmentDirectory(root, item.ID)
	if err != nil {
		return err
	}
	defer dir.Close()
	for _, file := range files {
		name := attachmentFilename(file.ID)
		info, err := dir.Lstat(name)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("attachment %s is missing or not a regular file", file.ID)
		}
		source, err := dir.Open(name)
		if err != nil {
			return fmt.Errorf("cannot open attachment %s: %w", file.ID, err)
		}
		err = decryptAttachment(key, io.Discard, source, file)
		_ = source.Close()
		if err != nil {
			return fmt.Errorf("attachment %s failed integrity verification: %w", file.ID, err)
		}
	}
	return nil
}

// DeleteItem removes a CXF item and its vault summary.
func (s *FileStorage) DeleteItem(vault *model.Vault, itemID string) error {
	return s.deleteItem(vault, itemID)
}

// storeItem persists an immutable version before committing its summary in the vault.
func (s *FileStorage) storeItem(vault *model.Vault, item *model.Item) error {
	if err := vault.Validate(); err != nil {
		return fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return fmt.Errorf("could not store item: vault key is missing")
	}
	if err := item.Validate(); err != nil {
		return fmt.Errorf("could not prepare item: %w", err)
	}
	id := vault.Accounts[0].ID
	root, err := s.openVaultRoot(id)
	if err != nil {
		return fmt.Errorf("could not open vault: %w", err)
	}
	defer root.Close()

	updated, err := prepareItemVault(vault, item)
	if err != nil {
		return err
	}
	stateHash, err := item.StateHash()
	if err != nil {
		return fmt.Errorf("could not calculate item state hash: %w", err)
	}
	itemDir := filepath.Join(itemsDirectoryName, filesystemID(item.ID))
	if err := root.MkdirAll(itemDir, 0o700); err != nil {
		return fmt.Errorf("could not create item directory: %w", err)
	}
	newPath := itemRelativePath(item.ID, stateHash)
	if _, err := root.Lstat(newPath); errors.Is(err, os.ErrNotExist) {
		if err := s.storeEncryptedInRoot(root, newPath, vault.Key(), item); err != nil {
			return fmt.Errorf("could not store item: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("could not inspect item version: %w", err)
	} else if err := verifyItemVersion(root, newPath, vault, item.ID, stateHash); err != nil {
		return fmt.Errorf("existing Item version collision: %w", err)
	}
	for _, dir := range []string{itemDir, itemsDirectoryName, "."} {
		if err := syncDirectory(root, dir); err != nil {
			return fmt.Errorf("persist Item directory: %w", err)
		}
	}
	oldHash, err := summaryStateHashForID(vault, item.ID)
	if err != nil {
		return err
	}
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was saved: %w", err)
	}
	*vault = *updated
	if err := syncDirectory(root, "."); err != nil {
		log.Printf("storage: Vault committed but directory sync failed; preserving Item versions: %v", err)
		return nil
	}
	if oldHash != "" && oldHash != stateHash {
		s.removeItemVersionBestEffort(root, item.ID, oldHash)
	}
	s.cleanupItemAttachmentsBestEffort(root, item)
	s.cleanupItemVersionsBestEffort(root, vault)
	return nil
}

// deleteItem commits removal of the summary before cleaning up physical versions.
func (s *FileStorage) deleteItem(vault *model.Vault, itemID string) error {
	updated, err := prepareItemDeleteVault(vault, itemID)
	if err != nil {
		return err
	}
	id := updated.Accounts[0].ID
	root, err := s.openVaultRoot(id)
	if err != nil {
		return fmt.Errorf("could not open vault: %w", err)
	}
	defer root.Close()
	updated.Timestamp = uint64(time.Now().Unix())
	if err := s.storeEncryptedInRoot(root, vaultFileName, vault.Key(), updated); err != nil {
		return fmt.Errorf("could not store vault after item was deleted: %w", err)
	}
	*vault = *updated
	if err := syncDirectory(root, "."); err != nil {
		log.Printf("storage: Vault committed but directory sync failed; preserving deleted Item files: %v", err)
		return nil
	}
	s.cleanupItemVersionsBestEffort(root, vault)
	return nil
}

// vaultHasItem checks whether an item summary belongs to the vault.
func vaultHasItem(vault *model.Vault, itemID string) bool {
	if vault == nil || len(vault.Accounts) != 1 {
		return false
	}
	for _, item := range vault.Accounts[0].Items {
		if item.ID == itemID {
			return true
		}
	}
	return false
}

func vaultItemSummary(vault *model.Vault, itemID string) (cxf.Item, bool) {
	if vault == nil || len(vault.Accounts) != 1 {
		return cxf.Item{}, false
	}
	for _, item := range vault.Accounts[0].Items {
		if item.ID == itemID {
			return item, true
		}
	}
	return cxf.Item{}, false
}

func summaryStateHash(summary cxf.Item) (string, error) {
	result := ""
	for _, raw := range summary.Extensions {
		var value struct {
			Name      string `json:"name"`
			Hash      string `json:"hash"`
			Algorithm string `json:"algorithm"`
		}
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		if value.Name == itemHashExtensionName {
			if result != "" || value.Algorithm != itemHashAlgorithm || !validStateHash(value.Hash) {
				return "", fmt.Errorf("item %q has an invalid or duplicate StateHash", summary.ID)
			}
			result = value.Hash
		}
	}
	if result != "" {
		return result, nil
	}
	return "", fmt.Errorf("item %q has a missing or invalid StateHash: %w", summary.ID, errMissingStateHash)
}

func validStateHash(value string) bool {
	if len(value) != 64 || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func summaryStateHashForID(vault *model.Vault, itemID string) (string, error) {
	summary, ok := vaultItemSummary(vault, itemID)
	if !ok {
		return "", nil
	}
	return summaryStateHash(summary)
}

func (s *FileStorage) removeItemVersionBestEffort(root *os.Root, itemID, hash string) {
	if err := root.Remove(itemRelativePath(itemID, hash)); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("storage cleanup: remove obsolete Item version %q: %v", itemID, err)
	}
}

// cleanupItemAttachmentsBestEffort removes only canonical files unused by current or retained history.
func (s *FileStorage) cleanupItemAttachmentsBestEffort(root *os.Root, item *model.Item) {
	if !isVaultID(item.ID) {
		return
	}
	references := make(map[string]struct{})
	addFiles := func(snapshot *model.Item) error {
		files, err := snapshot.Files()
		if err != nil {
			return err
		}
		for _, file := range files {
			references[file.ID] = struct{}{}
		}
		return nil
	}
	if err := addFiles(item); err != nil {
		log.Printf("storage cleanup: preserve attachments for Item %q: %v", item.ID, err)
		return
	}
	history, err := item.History()
	if err != nil {
		log.Printf("storage cleanup: preserve attachments for Item %q: %v", item.ID, err)
		return
	}
	for _, entry := range history.Entries {
		data, err := json.Marshal(entry.Item)
		if err != nil {
			return
		}
		var snapshot model.Item
		if err := json.Unmarshal(data, &snapshot); err != nil || addFiles(&snapshot) != nil {
			log.Printf("storage cleanup: preserve attachments for Item %q: invalid history references", item.ID)
			return
		}
	}
	dir, err := openAttachmentDirectory(root, item.ID)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err != nil {
		log.Printf("storage cleanup: open attachments for Item %q: %v", item.ID, err)
		return
	}
	defer dir.Close()
	listing, err := dir.Open(".")
	if err != nil {
		return
	}
	entries, err := listing.ReadDir(-1)
	_ = listing.Close()
	if err != nil {
		return
	}
	for _, entry := range entries {
		name := entry.Name()
		if isAttachmentTempName(name) {
			info, err := dir.Lstat(name)
			if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
				if err := dir.Remove(name); err != nil {
					log.Printf("storage cleanup: remove staged attachment for Item %q: %v", item.ID, err)
				}
			}
			continue
		}
		if !strings.HasSuffix(name, ".age") || entry.Type() != 0 {
			continue
		}
		fileID, ok := idFromFilesystemName(strings.TrimSuffix(name, ".age"))
		if !ok {
			continue
		}
		if _, ok := references[fileID]; ok {
			continue
		}
		if err := dir.Remove(name); err != nil {
			log.Printf("storage cleanup: remove unreferenced attachment for Item %q: %v", item.ID, err)
		}
	}
}

func isAttachmentTempName(name string) bool {
	const prefix, suffix = ".paw-attachment-", ".tmp"
	if len(name) != len(prefix)+24+len(suffix) || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return false
	}
	token := name[len(prefix) : len(prefix)+24]
	if strings.ToLower(token) != token {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}

func (s *FileStorage) cleanupItemVersionsBestEffort(root *os.Root, vault *model.Vault) {
	if err := syncDirectory(root, "."); err != nil {
		log.Printf("storage cleanup: preserve versions after directory sync failure: %v", err)
		return
	}
	dir, err := root.Open(itemsDirectoryName)
	if err != nil {
		log.Printf("storage cleanup: open Item directories: %v", err)
		return
	}
	entries, err := dir.ReadDir(-1)
	_ = dir.Close()
	if err != nil {
		log.Printf("storage cleanup: list Item directories: %v", err)
		return
	}
	for _, entry := range entries {
		itemID, ok := idFromFilesystemName(entry.Name())
		if !ok || !entry.IsDir() {
			continue
		}
		summary, indexed := vaultItemSummary(vault, itemID)
		keep := ""
		if indexed {
			keep, err = summaryStateHash(summary)
			if err != nil {
				log.Printf("storage cleanup: skip Item %q: %v", itemID, err)
				continue
			}
		}
		itemDir, openErr := root.Open(filepath.Join(itemsDirectoryName, entry.Name()))
		if openErr != nil {
			log.Printf("storage cleanup: open Item directory %q: %v", itemID, openErr)
			continue
		}
		files, readErr := itemDir.ReadDir(-1)
		_ = itemDir.Close()
		if readErr != nil {
			log.Printf("storage cleanup: list Item versions %q: %v", itemID, readErr)
			continue
		}
		for _, file := range files {
			name := file.Name()
			if !strings.HasSuffix(name, ".age") || !validStateHash(strings.TrimSuffix(name, ".age")) || file.Type() != 0 {
				continue
			}
			if name == keep+".age" {
				continue
			}
			if removeErr := root.Remove(filepath.Join(itemsDirectoryName, entry.Name(), name)); removeErr != nil {
				log.Printf("storage cleanup: remove obsolete Item version %q: %v", itemID, removeErr)
			}
		}
		if !indexed {
			path := filepath.Join(itemsDirectoryName, entry.Name())
			s.cleanupDeletedItemAttachmentsBestEffort(root, path)
			dir, err := root.Open(path)
			if err != nil {
				log.Printf("storage cleanup: open directory %q: %v", itemID, err)
				continue
			}
			remaining, err := dir.ReadDir(-1)
			_ = dir.Close()
			if err != nil {
				log.Printf("storage cleanup: inspect directory %q: %v", itemID, err)
			} else if len(remaining) == 0 {
				if err := root.Remove(path); err != nil {
					log.Printf("storage cleanup: remove empty directory %q: %v", itemID, err)
				}
			}
		}
	}
}

// cleanupDeletedItemAttachmentsBestEffort removes recognized blobs after the Item summary is gone.
func (s *FileStorage) cleanupDeletedItemAttachmentsBestEffort(root *os.Root, itemPath string) {
	itemRoot, err := root.OpenRoot(itemPath)
	if err != nil {
		return
	}
	info, err := itemRoot.Lstat(filesDirectoryName)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		_ = itemRoot.Close()
		return
	}
	filesRoot, err := itemRoot.OpenRoot(filesDirectoryName)
	if err != nil {
		_ = itemRoot.Close()
		return
	}
	listing, err := filesRoot.Open(".")
	if err != nil {
		_ = filesRoot.Close()
		_ = itemRoot.Close()
		return
	}
	entries, err := listing.ReadDir(-1)
	_ = listing.Close()
	if err == nil {
		for _, entry := range entries {
			name := entry.Name()
			if isAttachmentTempName(name) {
				info, err := filesRoot.Lstat(name)
				if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
					_ = filesRoot.Remove(name)
				}
				continue
			}
			if !strings.HasSuffix(name, ".age") || entry.Type() != 0 {
				continue
			}
			if _, ok := idFromFilesystemName(strings.TrimSuffix(name, ".age")); ok {
				_ = filesRoot.Remove(name)
			}
		}
	}
	_ = filesRoot.Close()
	_ = itemRoot.Close()
	_ = root.Remove(filepath.Join(itemPath, filesDirectoryName))
}

// verifyItemVersion checks an existing immutable file before it is reused.
func verifyItemVersion(root *os.Root, name string, vault *model.Vault, id, hash string) error {
	info, err := root.Lstat(name)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("item version is not a regular file: %v", err)
	}
	f, err := root.Open(name)
	if err != nil {
		return err
	}
	item, readErr := decryptItem(vault.Key(), f, id)
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return err
	}
	actual, err := item.StateHash()
	if err != nil || actual != hash {
		return fmt.Errorf("item version StateHash mismatch")
	}
	return nil
}

// prepareItemVault returns a separate vault with the item's derived summary.
func prepareItemVault(vault *model.Vault, item *model.Item) (*model.Vault, error) {
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("could not store item: vault key is missing")
	}
	summary, err := item.Summary()
	if err != nil {
		return nil, fmt.Errorf("could not prepare item summary: %w", err)
	}
	stateHash, err := item.StateHash()
	if err != nil {
		return nil, fmt.Errorf("could not calculate item state hash: %w", err)
	}
	hashExtension, err := json.Marshal(struct {
		Name      string `json:"name"`
		Algorithm string `json:"algorithm"`
		Hash      string `json:"hash"`
	}{
		Name:      itemHashExtensionName,
		Algorithm: itemHashAlgorithm,
		Hash:      stateHash,
	}, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("could not encode item state hash: %w", err)
	}
	summary.Extensions = append(summary.Extensions, hashExtension)

	updated := *vault
	updated.Accounts = slices.Clone(vault.Accounts)
	account := &updated.Accounts[0]
	account.Items = slices.Clone(account.Items)
	// Vault marshaling replaces its Paw extension, so isolate that slice too.
	account.Extensions = slices.Clone(account.Extensions)
	index := slices.IndexFunc(account.Items, func(candidate cxf.Item) bool {
		return candidate.ID == item.ID
	})
	if index < 0 {
		account.Items = append(account.Items, *summary)
	} else {
		account.Items[index] = *summary
	}
	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare updated vault: %w", err)
	}
	return &updated, nil
}

// prepareItemDeleteVault returns a separate vault without the selected item summary.
func prepareItemDeleteVault(vault *model.Vault, itemID string) (*model.Vault, error) {
	if err := vault.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare vault: %w", err)
	}
	if vault.Key() == nil {
		return nil, fmt.Errorf("could not delete item: vault key is missing")
	}

	updated := *vault
	updated.Accounts = slices.Clone(vault.Accounts)
	account := &updated.Accounts[0]
	account.Items = slices.Clone(account.Items)
	account.Extensions = slices.Clone(account.Extensions)
	index := slices.IndexFunc(account.Items, func(candidate cxf.Item) bool {
		return candidate.ID == itemID
	})
	if index < 0 {
		return nil, fmt.Errorf("could not delete item: item %q is not in the vault", itemID)
	}
	account.Items = slices.Delete(account.Items, index, index+1)
	if err := updated.Validate(); err != nil {
		return nil, fmt.Errorf("could not prepare updated vault: %w", err)
	}
	return &updated, nil
}
