// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.lucor.dev/paw/internal/paw"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
)

func TestDocumentSaveVerifiesAndPublishes(t *testing.T) {
	s, root, itemID, key := newDocumentSaveRoot(t)
	fileID := testDocumentID(t)
	content := []byte{0, 255, 2, 0, 19}
	size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader(content))
	require.NoError(t, err)
	expected := cxf.File{Type: cxf.CredentialTypeFile, ID: fileID, Name: "original.bin", DecryptedSize: size, IntegrityHash: digest}
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "folder with spaces", "saved file.bin")
	require.NoError(t, os.Mkdir(filepath.Dir(destination), 0700))
	require.NoError(t, s.saveDocument(root, key, itemID, fileID, destination, expected))
	got, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, content, got)
	info, err := os.Stat(destination)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	entries, err := os.ReadDir(filepath.Dir(destination))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestDocumentSavePublishesEmptyDocument(t *testing.T) {
	s, root, itemID, key := newDocumentSaveRoot(t)
	fileID := testDocumentID(t)
	size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader(nil))
	require.NoError(t, err)
	expected := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "empty.bin",
		DecryptedSize: size,
		IntegrityHash: digest,
	}
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "empty.bin")
	require.NoError(t, s.saveDocument(root, key, itemID, fileID, destination, expected))
	info, err := os.Stat(destination)
	require.NoError(t, err)
	require.Zero(t, info.Size())
}

func TestDocumentSavePreservesExistingDestinationAndCleansFailure(t *testing.T) {
	s, root, itemID, key := newDocumentSaveRoot(t)
	fileID := testDocumentID(t)
	content := []byte("verified output")
	size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader(content))
	require.NoError(t, err)
	expected := cxf.File{Type: cxf.CredentialTypeFile, ID: fileID, Name: "private name", DecryptedSize: size, IntegrityHash: digest}
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "existing")
	require.NoError(t, os.WriteFile(destination, []byte("sentinel"), 0600))
	require.Error(t, s.saveDocument(root, key, itemID, fileID, destination, expected))
	got, err := os.ReadFile(destination)
	require.NoError(t, err)
	require.Equal(t, []byte("sentinel"), got)

	bad := expected
	bad.IntegrityHash = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	failedPath := filepath.Join(destinationDir, "failed")
	require.Error(t, s.saveDocument(root, key, itemID, fileID, failedPath, bad))
	require.NoFileExists(t, failedPath)
	storageDirectory := storageRootPath(s)
	vaultDirectory, err := filepath.EvalSymlinks(storageDirectory)
	require.NoError(t, err)
	require.NoError(t, os.Mkdir(filepath.Join(vaultDirectory, "another-vault"), 0700))
	vaultPath := filepath.Join(vaultDirectory, "another-vault", "plaintext")
	require.Error(t, s.saveDocument(root, key, itemID, fileID, vaultPath, expected))
	require.NoFileExists(t, vaultPath)
	entries, err := os.ReadDir(filepath.Dir(failedPath))
	require.NoError(t, err)
	for _, entry := range entries {
		require.NotContains(t, entry.Name(), ".paw-save-")
	}
}

func TestDocumentSaveCleansFailures(t *testing.T) {
	for _, test := range []struct {
		name      string
		wrongKey  bool
		truncated bool
		badSize   bool
	}{
		{name: "wrong key", wrongKey: true},
		{name: "truncated ciphertext", truncated: true},
		{name: "size mismatch", badSize: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, root, itemID, key := newDocumentSaveRoot(t)
			fileID := testDocumentID(t)
			size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader([]byte("attachment data")))
			require.NoError(t, err)
			expected := cxf.File{Type: cxf.CredentialTypeFile, ID: fileID, Name: "file.bin", DecryptedSize: size, IntegrityHash: digest}
			if test.badSize {
				expected.DecryptedSize++
			}
			if test.truncated {
				path := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName, documentFilename(fileID))
				ciphertext, err := root.ReadFile(path)
				require.NoError(t, err)
				require.NoError(t, root.WriteFile(path, ciphertext[:len(ciphertext)-8], 0600))
			}
			if test.wrongKey {
				key, err = paw.MakeOneTimeKey()
				require.NoError(t, err)
			}
			dir, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			require.Error(t, s.saveDocument(root, key, itemID, fileID, filepath.Join(dir, "result"), expected))
			entries, err := os.ReadDir(dir)
			require.NoError(t, err)
			for _, entry := range entries {
				require.False(t, strings.HasPrefix(entry.Name(), ".paw-save-"))
			}
		})
	}
}

func TestDocumentSaveRejectsSymlinkDestinationParent(t *testing.T) {
	s, root, itemID, key := newDocumentSaveRoot(t)
	fileID := testDocumentID(t)
	size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader([]byte("data")))
	require.NoError(t, err)
	expected := cxf.File{Type: cxf.CredentialTypeFile, ID: fileID, Name: "file.bin", DecryptedSize: size, IntegrityHash: digest}
	base, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	target := filepath.Join(base, "target")
	require.NoError(t, os.Mkdir(target, 0700))
	link := filepath.Join(base, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	require.Error(t, s.saveDocument(root, key, itemID, fileID, filepath.Join(link, "result"), expected))
	require.NoFileExists(t, filepath.Join(target, "result"))
}

func TestDocumentSaveStreamsGeneratedLargeContent(t *testing.T) {
	s, root, itemID, key := newDocumentSaveRoot(t)
	fileID := testDocumentID(t)
	const size = 16 * 1024 * 1024
	sizeOnDisk, digest, err := storeDocument(root, key, itemID, fileID, &patternReader{remaining: size})
	require.NoError(t, err)
	expected := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "large.bin",
		DecryptedSize: sizeOnDisk,
		IntegrityHash: digest,
	}
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	destination := filepath.Join(destinationDir, "large.bin")
	require.NoError(t, s.saveDocument(root, key, itemID, fileID, destination, expected))
	file, err := os.Open(destination)
	require.NoError(t, err)
	hash := sha256.New()
	actualSize, err := io.Copy(hash, file)
	require.NoError(t, err)
	require.NoError(t, file.Close())
	require.Equal(t, int64(size), actualSize)
	require.Equal(t, digest, base64.RawURLEncoding.EncodeToString(hash.Sum(nil)))
}

func TestDocumentSaveRejectsCorruptionWithoutPublishingTemporary(t *testing.T) {
	s, root, itemID, key := newDocumentSaveRoot(t)
	fileID := testDocumentID(t)
	content := []byte("encrypted content")
	size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader(content))
	require.NoError(t, err)
	expected := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "file.bin",
		DecryptedSize: size,
		IntegrityHash: digest,
	}
	documents := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName)
	path := filepath.Join(documents, documentFilename(fileID))
	ciphertext, err := root.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, root.WriteFile(path, tamperDocument(ciphertext), 0600))
	destinationDir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	corruptDestination := filepath.Join(destinationDir, "corrupt.bin")
	require.Error(t, s.saveDocument(root, key, itemID, fileID, corruptDestination, expected))
	require.NoFileExists(t, corruptDestination)
	entries, err := os.ReadDir(destinationDir)
	require.NoError(t, err)
	require.Empty(t, entries)

}

// newDocumentSaveRoot creates the real application/storage/vault layout without password derivation.
func newDocumentSaveRoot(t *testing.T) (*FileStorage, *os.Root, string, *paw.Key) {
	t.Helper()
	s, err := NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	vaultID := testDocumentID(t)
	require.NoError(t, s.root.Mkdir(vaultDirectory(vaultID), 0700))
	root, err := s.openVaultRoot(vaultID)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	itemID := testDocumentID(t)
	require.NoError(t, root.MkdirAll(filepath.Join(itemsDirectoryName, filesystemID(itemID)), 0700))
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	return s, root, itemID, key
}
