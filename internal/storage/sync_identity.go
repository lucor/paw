// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"crypto/mldsa"
	"errors"
	"fmt"
	"io"
	"os"

	"go.lucor.dev/paw/internal/model"
)

const syncIdentityFileName = "sync.age"

// LoadOrCreateSyncIdentity loads or creates the device-specific identity for an unlocked vault.
func (s *FileStorage) LoadOrCreateSyncIdentity(vault *model.Vault) (*mldsa.PublicKey, *mldsa.PrivateKey, error) {
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	data, err := root.ReadFile(syncIdentityFileName)
	if err == nil {
		privateKey, err := decryptSyncIdentity(vault, data)
		if err != nil {
			return nil, nil, err
		}
		return privateKey.PublicKey(), privateKey, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("could not read Sync identity: %w", err)
	}
	privateKey, err := mldsa.GenerateKey(mldsa.MLDSA44())
	if err != nil {
		return nil, nil, fmt.Errorf("could not generate Sync identity: %w", err)
	}
	if err := writeSyncIdentity(root, vault, privateKey); err != nil {
		return nil, nil, err
	}
	return privateKey.PublicKey(), privateKey, nil
}

// StoreSyncIdentity saves a previously generated identity for a new vault.
func (s *FileStorage) StoreSyncIdentity(vault *model.Vault, privateKey *mldsa.PrivateKey) error {
	if privateKey == nil {
		return fmt.Errorf("sync identity is required")
	}
	root, err := s.syncVaultRoot(vault)
	if err != nil {
		return err
	}
	defer root.Close()
	if _, err := root.Lstat(syncIdentityFileName); err == nil {
		return fmt.Errorf("sync identity already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("could not check Sync identity: %w", err)
	}
	return writeSyncIdentity(root, vault, privateKey)
}

func writeSyncIdentity(root *os.Root, vault *model.Vault, privateKey *mldsa.PrivateKey) error {
	privateKeyData := privateKey.Bytes()
	defer clear(privateKeyData)
	if err := atomicWrite(root, syncIdentityFileName, func(w io.Writer) error {
		writer, err := vault.Key().Encrypt(w)
		if err != nil {
			return err
		}
		if _, err := writer.Write(privateKeyData); err != nil {
			return errors.Join(err, writer.Close())
		}
		return writer.Close()
	}); err != nil {
		return fmt.Errorf("could not store Sync identity: %w", err)
	}
	return nil
}

func decryptSyncIdentity(vault *model.Vault, encrypted []byte) (*mldsa.PrivateKey, error) {
	data, err := readDecrypted(vault.Key(), bytes.NewReader(encrypted))
	if err != nil {
		return nil, fmt.Errorf("could not decrypt Sync identity: %w", err)
	}
	defer clear(data)
	if len(data) != mldsa.PrivateKeySize {
		return nil, fmt.Errorf("invalid Sync identity")
	}
	privateKey, err := mldsa.NewPrivateKey(mldsa.MLDSA44(), data)
	if err != nil {
		return nil, fmt.Errorf("invalid Sync identity: %w", err)
	}
	return privateKey, nil
}
