// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/base32"
	"encoding/base64"
	"path/filepath"
	"runtime"
	"strings"
)

// SocketAgentPath returns the socket agent path.
func (s *FileStorage) SocketAgentPath() string { return socketAgentPath(s) }

// LockFilePath returns the lock file path.
func (s *FileStorage) LockFilePath() string { return lockFilePath(s) }

// LogFilePath returns the log file path.
func (s *FileStorage) LogFilePath() string { return logFilePath(s) }

const (
	storageRootName        = "storage"
	vaultBackupSuffix      = ".bck"
	keyFileName            = "key.age"
	vaultFileName          = "vault.age"
	itemsDirectoryName     = "items"
	documentsDirectoryName = "documents"
	vaultMetadataFileName  = "metadata.json"
	appStateFileName       = "paw.json"
	lockFileName           = "paw.lock"
	logFileName            = "paw.log"
	socketFileName         = "agent.sock"
	namedPipe              = `\\.\pipe\paw`
)

// storageRootPath returns the directory containing vaults.
func storageRootPath(s *FileStorage) string {
	return filepath.Join(s.Root(), storageRootName)
}

// appStateFilePath returns the application settings file path.
func appStateFilePath(s *FileStorage) string {
	return filepath.Join(s.Root(), appStateFileName)
}

// vaultRootPath returns the directory containing one vault's files.
func vaultRootPath(s *FileStorage, vaultID string) string {
	return filepath.Join(s.Root(), vaultDirectory(vaultID))
}

// vaultDirectory returns the application-relative directory for a validated vault ID.
func vaultDirectory(vaultID string) string {
	return filepath.Join(storageRootName, filesystemID(vaultID))
}

// keyPath returns the encrypted vault key file path.
func keyPath(s *FileStorage, vaultID string) string {
	return filepath.Join(vaultRootPath(s, vaultID), keyFileName)
}

// vaultPath returns the encrypted vault index file path.
func vaultPath(s *FileStorage, vaultID string) string {
	return filepath.Join(vaultRootPath(s, vaultID), vaultFileName)
}

// vaultMetadataPath returns the plaintext listing metadata path for one vault.
func vaultMetadataPath(s *FileStorage, vaultID string) string {
	return filepath.Join(vaultRootPath(s, vaultID), vaultMetadataFileName)
}

// itemPath returns an encrypted item's file path.
func itemPath(s *FileStorage, vaultID string, itemID, stateHash string) string {
	return filepath.Join(vaultRootPath(s, vaultID), itemRelativePath(itemID, stateHash))
}

// itemRelativePath returns an encrypted item's path relative to its vault root.
func itemRelativePath(itemID, stateHash string) string {
	return filepath.Join(itemsDirectoryName, filesystemID(itemID), stateHash+".age")
}

// documentFilename returns the age filename derived from a canonical file ID.
func documentFilename(fileID string) string {
	return filesystemID(fileID) + ".age"
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
func socketAgentPath(s *FileStorage) string {
	if runtime.GOOS == "windows" {
		return namedPipe
	}
	return filepath.Join(s.Root(), socketFileName)
}

// lockFilePath returns the application lock file path.
func lockFilePath(s *FileStorage) string {
	return filepath.Join(s.Root(), lockFileName)
}

// logFilePath returns the application log file path.
func logFilePath(s *FileStorage) string {
	return filepath.Join(s.Root(), logFileName)
}

// isVaultID reports whether value is a canonical Paw 128-bit Base64url ID.
func isVaultID(value string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(decoded) == 16 && base64.RawURLEncoding.EncodeToString(decoded) == value
}
