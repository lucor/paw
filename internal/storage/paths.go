// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"encoding/base32"
	"encoding/base64"
	"fmt"
	"io"
	"os"
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
	storageRootName         = "storage"
	vaultBackupSuffix       = ".bck"
	keyFileName             = "key.age"
	identitiesDirectoryName = "identities"
	identityFileName        = "scrypt.age"
	vaultFileName           = "vault.age"
	itemsDirectoryName      = "items"
	filesDirectoryName      = "files"
	vaultMetadataFileName   = "metadata.json"
	clonePendingFileName    = "clone-pending"
	appStateFileName        = "paw.json"
	lockFileName            = "paw.lock"
	logFileName             = "paw.log"
	socketFileName          = "agent.sock"
	namedPipe               = `\\.\pipe\paw`
)

// storageRootPath returns the directory containing vaults.
func storageRootPath(s *FileStorage) string {
	return filepath.Join(s.Root(), storageRootName)
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
	return filepath.Join(vaultRootPath(s, vaultID), scryptIdentityPath())
}

// scryptIdentityPath returns the password-protected identity path within a vault.
func scryptIdentityPath() string {
	return filepath.Join(identitiesDirectoryName, identityFileName)
}

func legacyKeyPath(s *FileStorage, vaultID string) string {
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

// attachmentFilename returns the age filename derived from a canonical file ID.
func attachmentFilename(fileID string) string {
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

// FormatVaultID returns the user-facing form of a canonical Vault ID.
func FormatVaultID(id string) string {
	return filesystemID(id)
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

// OpenLogFile opens the application log in append mode, or the configured override.
func (s *FileStorage) OpenLogFile(override string) (io.WriteCloser, error) {
	path := logFilePath(s)
	if override != "" {
		path = override
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
}

// PrepareAgentSocket removes a stale socket and ensures its parent directory exists.
func (s *FileStorage) PrepareAgentSocket() (string, error) {
	path := socketAgentPath(s)
	if runtime.GOOS == "windows" {
		return path, nil
	}
	if err := s.root.Remove(socketFileName); err != nil && !os.IsNotExist(err) {
		return "", err
	}
	return path, nil
}

// WriteNativeManifest writes a browser integration manifest to its OS-selected location.
func (s *FileStorage) WriteNativeManifest(path string, data []byte) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("native manifest path must be absolute")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// isVaultID reports whether value is a canonical Paw 128-bit Base64url ID.
func isVaultID(value string) bool {
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(value)
	return err == nil && len(decoded) == 16 && base64.RawURLEncoding.EncodeToString(decoded) == value
}
