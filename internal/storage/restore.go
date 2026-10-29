// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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

func (r backupReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

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
		encoded, err := json.Marshal(raw)
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
	index, err := json.Marshal(document)
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

// BackupCollision reports any existing directory using the backup's logical identity.
func BackupCollision(s Storage, backup *PawBackup) (bool, error) {
	if backup == nil || !isVaultID(backup.AccountID) {
		return false, ErrUnsupportedBackup
	}
	_, err := os.Lstat(vaultRootPath(s, backup.AccountID))
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

// RestorePawBackup creates a private encrypted vault, publishing its catalog last.
// Existing vaults are never changed; an occupied account ID creates an independent copy.
func RestorePawBackup(ctx context.Context, s Storage, backup *PawBackup, name, password string) (result *model.Vault, err error) {
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
		err = os.Mkdir(vaultRootPath(s, id), 0700)
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
		if _, err := os.Lstat(vaultRootPath(s, id)); !os.IsNotExist(err) {
			return nil, fmt.Errorf("restored copy identity is already occupied")
		}
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(vaultRootPath(s, id)))
		}
	}()
	if id != oldID {
		source.Accounts[0].ID = id
		source.Metadata.CreationAt = fresh.Metadata.CreationAt
		source.Timestamp = fresh.Timestamp
		remapBackupCollections(source.Accounts[0].Collections, oldID, id)
	}
	source.Metadata.VaultName = name
	index, err := json.Marshal(source)
	if err != nil {
		return nil, err
	}
	restored, err := model.DecodeVault(index, fresh.Key())
	if err != nil {
		return nil, err
	}
	if err = os.WriteFile(keyPath(s, id), keyData, 0600); err != nil {
		return nil, fmt.Errorf("write restored key: %w", err)
	}
	for _, item := range items {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		f, openErr := os.OpenFile(itemPath(s, id, item.ID), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if openErr != nil {
			return nil, openErr
		}
		if err = encryptFile(fresh.Key(), f, item); err != nil {
			return nil, fmt.Errorf("write restored item: %w", err)
		}
	}
	f, err := os.OpenFile(vaultPath(s, id), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	if err = encryptFile(fresh.Key(), f, restored); err != nil {
		return nil, fmt.Errorf("write restored vault: %w", err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	metadata, err := encodeVaultMetadata(name)
	if err != nil {
		return nil, err
	}
	// The canonical directory stays invisible to Vaults until this atomic publication.
	temp, err := os.CreateTemp(vaultRootPath(s, id), ".metadata-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(temp.Name())
	_, writeErr := temp.Write(metadata)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err = errors.Join(writeErr, syncErr, closeErr, ctx.Err()); err != nil {
		return nil, err
	}
	if err = os.Rename(temp.Name(), vaultMetadataPath(s, id)); err != nil {
		return nil, err
	}
	return restored, nil
}

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
