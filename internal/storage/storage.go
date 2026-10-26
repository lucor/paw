// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

// Package storage reads and writes Paw's encrypted files.
package storage

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strings"

	"go.lucor.dev/paw/internal/paw"
)

const (
	storageRootName       = "storage"
	vaultBackupSuffix     = ".bck"
	keyFileName           = "key.age"
	vaultFileName         = "vault.age"
	vaultMetadataFileName = "metadata.json"
	appStateFileName      = "paw.json"
	lockFileName          = "paw.lock"
	logFileName           = "paw.log"
	socketFileName        = "agent.sock"
	namedPipe             = `\\.\pipe\paw`
)

// storageRootPath returns the directory containing vaults.
func storageRootPath(s Storage) string {
	return filepath.Join(s.Root(), storageRootName)
}

// appStateFilePath returns the application settings file path.
func appStateFilePath(s Storage) string {
	return filepath.Join(s.Root(), appStateFileName)
}

// vaultRootPath returns the directory containing one vault's files.
func vaultRootPath(s Storage, vaultID string) string {
	return filepath.Join(storageRootPath(s), filesystemID(vaultID))
}

// keyPath returns the encrypted vault key file path.
func keyPath(s Storage, vaultID string) string {
	return filepath.Join(vaultRootPath(s, vaultID), keyFileName)
}

// vaultPath returns the encrypted vault index file path.
func vaultPath(s Storage, vaultID string) string {
	return filepath.Join(vaultRootPath(s, vaultID), vaultFileName)
}

// vaultMetadataPath returns the plaintext listing metadata path for one vault.
func vaultMetadataPath(s Storage, vaultID string) string {
	return filepath.Join(vaultRootPath(s, vaultID), vaultMetadataFileName)
}

// itemPath returns an encrypted item's file path.
func itemPath(s Storage, vaultID string, itemID string) string {
	itemFileName := fmt.Sprintf("%s.age", filesystemID(itemID))
	return filepath.Join(vaultRootPath(s, vaultID), itemFileName)
}

// filesystemID encodes a canonical Paw ID as lowercase, unpadded Base32.
func filesystemID(id string) string {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(id)
	if err != nil || len(decoded) != 16 || base64.RawURLEncoding.EncodeToString(decoded) != id {
		return ""
	}
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	return strings.ToLower(encoding.EncodeToString(decoded))
}

// idFromFilesystemName decodes a lowercase Base32 filesystem ID to Base64url.
func idFromFilesystemName(name string) (string, bool) {
	encoding := base32.StdEncoding.WithPadding(base32.NoPadding)
	decoded, err := encoding.DecodeString(strings.ToUpper(name))
	if err != nil || len(decoded) != 16 || strings.ToLower(encoding.EncodeToString(decoded)) != name {
		return "", false
	}
	return base64.RawURLEncoding.EncodeToString(decoded), true
}

// isFilesystemVaultID reports whether name is a canonical filesystem ID.
func isFilesystemVaultID(name string) bool {
	_, ok := idFromFilesystemName(name)
	return ok
}

// socketAgentPath returns the platform-specific address used by the agent.
func socketAgentPath(s Storage) string {
	if runtime.GOOS == "windows" {
		return namedPipe
	}
	return filepath.Join(s.Root(), socketFileName)
}

// lockFilePath returns the application lock file path.
func lockFilePath(s Storage) string {
	return filepath.Join(s.Root(), lockFileName)
}

// logFilePath returns the application log file path.
func logFilePath(s Storage) string {
	return filepath.Join(s.Root(), logFileName)
}

// encrypt writes a JSON value encrypted with the vault key.
func encrypt(key *paw.Key, w io.Writer, v any) error {
	encWriter, err := key.Encrypt(w)
	if err != nil {
		return fmt.Errorf("could not create encrypted writer for URI: %w", err)
	}

	encodeErr := json.NewEncoder(encWriter).Encode(v)
	if encodeErr != nil {
		encodeErr = fmt.Errorf("could not encode data for URI: %w", encodeErr)
	}
	closeErr := encWriter.Close()
	if closeErr != nil {
		closeErr = fmt.Errorf("could not close encrypted writer: %w", closeErr)
	}
	return errors.Join(encodeErr, closeErr)
}

// encryptFile writes encrypted JSON and closes the supplied file, including on failure.
func encryptFile(key *paw.Key, writer io.WriteCloser, value any) error {
	return errors.Join(encrypt(key, writer, value), writer.Close())
}
