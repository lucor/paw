// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"filippo.io/age"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

const maxBackupBytes = 64 << 20

// ErrBackupPasswordRequired means a binary age backup needs its export password.
var ErrBackupPasswordRequired = errors.New("backup password is required")

// ErrUnsupportedBackup identifies invalid or incompatible backup documents.
var ErrUnsupportedBackup = errors.New("this file is not a supported Paw backup")

// UnsupportedBackupMessage is the user-facing explanation for incompatible input.
const UnsupportedBackupMessage = "This file is not a supported Paw backup."

// PawBackup contains a validated document for review and restore, only in memory.
type PawBackup struct {
	data []byte
	// Name is the original vault display name.
	Name string
	// CreationAt is the optional original creation timestamp.
	CreationAt *uint64
	// ExportedAt is the timestamp carried by the exported CXF header.
	ExportedAt uint64
	// Collections counts collections, including nested collections.
	Collections int
	// Credentials counts CXF items, not individual fields within each item.
	Credentials int
	// AccountID is the original logical vault identity.
	AccountID string
}

// ReadPawBackup fully authenticates encrypted input and validates Paw metadata.
// Both input and decrypted content are limited to 64 MiB.
func ReadPawBackup(ctx context.Context, r io.Reader, password string) (*PawBackup, error) {
	data, err := readBackupBytes(ctx, r)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(data, []byte("age-encryption.org/v1\n")) {
		if password == "" {
			return nil, ErrBackupPasswordRequired
		}
		identity, err := age.NewScryptIdentity(password)
		if err != nil {
			return nil, fmt.Errorf("prepare backup decryption: %w", err)
		}
		// Paw exports use age's default factor of 18.
		identity.SetMaxWorkFactor(18)
		decrypted, err := age.Decrypt(bytes.NewReader(data), identity)
		if err != nil {
			return nil, fmt.Errorf("cannot unlock backup: incorrect password or invalid age file")
		}
		data, err = readBackupBytes(ctx, decrypted)
		if err != nil {
			return nil, fmt.Errorf("cannot authenticate backup: %w", err)
		}
	}
	backup, _, _, err := validatePawBackup(data)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return backup, err
}

// readBackupBytes reads a size-limited backup while honoring cancellation.
func readBackupBytes(ctx context.Context, r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(backupReader{ctx, r}, maxBackupBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read backup: %w", err)
	}
	if len(data) > maxBackupBytes {
		return nil, fmt.Errorf("backup exceeds the 64 MiB limit")
	}
	return data, ctx.Err()
}

type backupReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read checks for cancellation before reading backup data.
func (r backupReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// validatePawBackup decodes and validates a Paw CXF backup and its items.
func validatePawBackup(data []byte) (*PawBackup, *model.Vault, []*model.Item, error) {
	document, err := cxf.Decode(bytes.NewReader(data))
	if err != nil || len(document.Accounts) != 1 {
		return nil, nil, nil, ErrUnsupportedBackup
	}
	account := &document.Accounts[0]
	if !isVaultID(account.ID) {
		return nil, nil, nil, ErrUnsupportedBackup
	}
	items := make([]*model.Item, 0, len(account.Items))
	ids := map[string]bool{}
	for i, raw := range account.Items {
		encoded, err := json.Marshal(raw, json.Deterministic(true))
		if err != nil {
			return nil, nil, nil, ErrUnsupportedBackup
		}
		var item model.Item
		if json.Unmarshal(encoded, &item) != nil || item.Validate() != nil || !isVaultID(item.ID) {
			return nil, nil, nil, ErrUnsupportedBackup
		}
		if ids[item.ID] {
			return nil, nil, nil, ErrUnsupportedBackup
		}
		ids[item.ID] = true
		items = append(items, &item)
		summary, err := item.Summary()
		if err != nil {
			return nil, nil, nil, ErrUnsupportedBackup
		}
		account.Items[i] = *summary
	}
	count, err := checkBackupCollections(account.Collections, account.ID, ids)
	if err != nil {
		return nil, nil, nil, err
	}
	index, err := json.Marshal(document, json.Deterministic(true))
	if err != nil {
		return nil, nil, nil, ErrUnsupportedBackup
	}
	key, err := paw.MakeOneTimeKey()
	if err != nil {
		return nil, nil, nil, err
	}
	vault, err := model.DecodeVault(index, key)
	if err != nil || vault.Validate() != nil {
		return nil, nil, nil, ErrUnsupportedBackup
	}
	backup := &PawBackup{data: bytes.Clone(data), Name: vault.Metadata.VaultName, CreationAt: vault.Metadata.CreationAt, ExportedAt: document.Timestamp, Collections: count, Credentials: len(items), AccountID: account.ID}
	return backup, vault, items, nil
}

// checkBackupCollections validates collection links and counts nested collections.
func checkBackupCollections(collections []cxf.Collection, account string, ids map[string]bool) (int, error) {
	count := 0
	for _, collection := range collections {
		count++
		for _, link := range collection.Items {
			if !ids[link.Item] || (link.Account != nil && *link.Account != account) {
				return 0, ErrUnsupportedBackup
			}
		}
		children, err := checkBackupCollections(collection.SubCollections, account, ids)
		if err != nil {
			return 0, err
		}
		count += children
	}
	return count, nil
}

// Restore creates a private encrypted vault, publishing its catalog last.
// Existing vaults are never changed; an occupied account ID creates an independent copy.
func (s *FileStorage) Restore(ctx context.Context, backup *PawBackup, name, password string) (result *model.Vault, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if backup == nil {
		return nil, ErrUnsupportedBackup
	}
	_, source, items, err := validatePawBackup(backup.data)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" || !utf8.ValidString(name) {
		return nil, fmt.Errorf("vault name is required")
	}
	if password == "" {
		return nil, fmt.Errorf("new master password is required")
	}
	fresh, keyData, err := prepareVault(name, password)
	if err != nil {
		return nil, err
	}
	oldID := source.Accounts[0].ID
	id := oldID
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		err = s.root.Mkdir(vaultDirectory(id), 0700)
		if err == nil {
			break
		}
		if !os.IsExist(err) {
			return nil, fmt.Errorf("reserve restored vault: %w", err)
		}
		id = fresh.Accounts[0].ID
		if id == oldID {
			return nil, fmt.Errorf("cannot reserve an independent vault identity")
		}
		// A newly generated identity collision is an error, never an overwrite.
		if _, err := s.root.Lstat(vaultDirectory(id)); !os.IsNotExist(err) {
			return nil, fmt.Errorf("restored copy identity is already occupied")
		}
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, s.root.RemoveAll(vaultDirectory(id)))
		}
	}()
	if id != oldID {
		source.Accounts[0].ID = id
		source.Metadata.CreationAt = fresh.Metadata.CreationAt
		source.Timestamp = fresh.Timestamp
		remapBackupCollections(source.Accounts[0].Collections, oldID, id)
	}
	source.Metadata.VaultName = name
	index, err := json.Marshal(source, json.Deterministic(true))
	if err != nil {
		return nil, err
	}
	restored, err := model.DecodeVault(index, fresh.Key())
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		restored, err = prepareItemVault(restored, item)
		if err != nil {
			return nil, fmt.Errorf("prepare restored item summary: %w", err)
		}
	}
	vaultRoot, openErr := s.openVaultRoot(id)
	if openErr != nil {
		return nil, openErr
	}
	defer vaultRoot.Close()
	if err = vaultRoot.MkdirAll(itemsDirectoryName, 0o700); err != nil {
		return nil, fmt.Errorf("create restored item storage directory: %w", err)
	}
	if err = atomicWrite(vaultRoot, keyFileName, func(w io.Writer) error {
		_, writeErr := w.Write(keyData)
		return errors.Join(writeErr, ctx.Err())
	}); err != nil {
		return nil, fmt.Errorf("write restored key: %w", err)
	}
	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		itemDir := filepath.Join(itemsDirectoryName, filesystemID(item.ID))
		if err = vaultRoot.MkdirAll(itemDir, 0o700); err != nil {
			return nil, fmt.Errorf("create restored item directory: %w", err)
		}
		if err = atomicWrite(vaultRoot, itemRelativePath(item.ID), func(w io.Writer) error {
			return errors.Join(encrypt(fresh.Key(), w, item), ctx.Err())
		}); err != nil {
			return nil, fmt.Errorf("write restored item: %w", err)
		}
	}
	if err = atomicWrite(vaultRoot, vaultFileName, func(w io.Writer) error {
		return errors.Join(encrypt(fresh.Key(), w, restored), ctx.Err())
	}); err != nil {
		return nil, fmt.Errorf("write restored vault: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	metadata, err := encodeVaultMetadata(name, model.DefaultVaultAppearance())
	if err != nil {
		return nil, err
	}
	// The canonical directory stays invisible to Vaults until this atomic publication.
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	if err = atomicWrite(vaultRoot, vaultMetadataFileName, func(w io.Writer) error {
		_, writeErr := w.Write(metadata)
		return errors.Join(writeErr, ctx.Err())
	}); err != nil {
		return nil, err
	}
	return restored, nil
}

// remapBackupCollections updates collection links to the restored account ID.
func remapBackupCollections(collections []cxf.Collection, oldID, newID string) {
	for i := range collections {
		for j := range collections[i].Items {
			link := &collections[i].Items[j]
			if link.Account != nil && *link.Account == oldID {
				value := newID
				link.Account = &value
			}
		}
		remapBackupCollections(collections[i].SubCollections, oldID, newID)
	}
}
