// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestDocumentStorePublishesEncryptedFiles(t *testing.T) {
	for _, content := range [][]byte{[]byte("synthetic attachment"), {}, {0, 255, 1, 0, 42}} {
		t.Run(string(rune(len(content)+'0')), func(t *testing.T) {
			root, itemID, key := newDocumentStoreRoot(t)
			fileID := testDocumentID(t)
			size, digest, err := storeDocument(root, key, itemID, fileID, bytes.NewReader(content))
			require.NoError(t, err)
			require.Equal(t, uint64(len(content)), size)
			path := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName, documentFilename(fileID))
			info, err := root.Stat(path)
			require.NoError(t, err)
			require.True(t, info.Mode().IsRegular())
			if info.Mode().Perm() != 0 {
				require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			}
			file := cxf.File{
				Type:          cxf.CredentialTypeFile,
				ID:            fileID,
				Name:          "private-name.txt",
				DecryptedSize: size,
				IntegrityHash: digest,
			}
			ciphertext, err := root.Open(path)
			require.NoError(t, err)
			defer ciphertext.Close()
			var plaintext bytes.Buffer
			require.NoError(t, decryptDocument(key, &plaintext, ciphertext, file))
			require.True(t, bytes.Equal(content, plaintext.Bytes()))
			require.NoFileExists(t, filepath.Join(itemsDirectoryName, filesystemID(itemID), "item.age"))
			require.NoFileExists(t, vaultFileName)
			entries := readDocumentEntries(t, root, filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName))
			require.Len(t, entries, 1)
			documentsInfo, err := root.Stat(filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName))
			require.NoError(t, err)
			if documentsInfo.Mode().Perm() != 0 {
				require.Equal(t, os.FileMode(0700), documentsInfo.Mode().Perm())
			}
		})
	}
}

func TestDocumentCleanupPreservesAmbiguousEntriesAndIsIdempotent(t *testing.T) {
	root, itemID, _ := newDocumentStoreRoot(t)
	item, err := model.NewNote("cleanup", "")
	require.NoError(t, err)
	item.ID = itemID
	dir := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName)
	require.NoError(t, root.Mkdir(dir, 0700))
	orphan := testDocumentID(t)
	require.NoError(t, root.WriteFile(filepath.Join(dir, documentFilename(orphan)), []byte("orphan"), 0600))
	require.NoError(t, root.WriteFile(filepath.Join(dir, "unknown!.age"), []byte("keep"), 0600))
	require.NoError(t, root.WriteFile(filepath.Join(dir, "not-canonical!.age"), []byte("keep"), 0600))
	linkID := testDocumentID(t)
	if err := root.Symlink("unknown!.age", filepath.Join(dir, documentFilename(linkID))); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	dirID := testDocumentID(t)
	require.NoError(t, root.Mkdir(filepath.Join(dir, documentFilename(dirID)), 0700))
	storage := &FileStorage{}
	storage.cleanupItemDocumentsBestEffort(root, item)
	_, err = root.Lstat(filepath.Join(dir, documentFilename(orphan)))
	require.ErrorIs(t, err, os.ErrNotExist)
	for _, name := range []string{"unknown!.age", "not-canonical!.age"} {
		got, err := root.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		require.Equal(t, []byte("keep"), got)
	}
	info, err := root.Lstat(filepath.Join(dir, documentFilename(linkID)))
	require.NoError(t, err)
	require.True(t, info.Mode()&os.ModeSymlink != 0)
	info, err = root.Stat(filepath.Join(dir, documentFilename(dirID)))
	require.NoError(t, err)
	require.True(t, info.IsDir())
	storage.cleanupItemDocumentsBestEffort(root, item)
	_, err = root.Lstat(filepath.Join(dir, documentFilename(orphan)))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestDocumentCleanupPreservesFilesForMalformedHistory(t *testing.T) {
	root, itemID, _ := newDocumentStoreRoot(t)
	item, err := model.NewNote("cleanup", "")
	require.NoError(t, err)
	item.ID = itemID
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"paw.pm/history","schemaVersion":1,"entries":"invalid"}`))
	dir := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName)
	require.NoError(t, root.Mkdir(dir, 0700))
	orphan := filepath.Join(dir, documentFilename(testDocumentID(t)))
	require.NoError(t, root.WriteFile(orphan, []byte("preserve"), 0600))
	(&FileStorage{}).cleanupItemDocumentsBestEffort(root, item)
	got, err := root.ReadFile(orphan)
	require.NoError(t, err)
	require.Equal(t, []byte("preserve"), got)
}

func TestDocumentStoreRejectsExistingTargets(t *testing.T) {
	for _, kind := range []string{"regular", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root, itemID, key := newDocumentStoreRoot(t)
			fileID := testDocumentID(t)
			documents := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName)
			require.NoError(t, root.Mkdir(documents, 0700))
			name := documentFilename(fileID)
			switch kind {
			case "regular":
				require.NoError(t, root.WriteFile(filepath.Join(documents, name), []byte("sentinel"), 0600))
			case "directory":
				require.NoError(t, root.Mkdir(filepath.Join(documents, name), 0700))
			case "symlink":
				outside := filepath.Join(t.TempDir(), "sentinel")
				require.NoError(t, os.WriteFile(outside, []byte("sentinel"), 0600))
				if err := root.Symlink(outside, filepath.Join(documents, name)); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			_, _, err := storeDocument(root, key, itemID, fileID, bytes.NewReader([]byte("content")))
			require.Error(t, err)
			entries := readDocumentEntries(t, root, documents)
			require.Len(t, entries, 1)
			if kind == "regular" {
				got, err := root.ReadFile(filepath.Join(documents, name))
				require.NoError(t, err)
				require.Equal(t, []byte("sentinel"), got)
			}
		})
	}
}

func TestDocumentStoreRejectsSameContentCollision(t *testing.T) {
	root, itemID, key := newDocumentStoreRoot(t)
	fileID := testDocumentID(t)
	content := []byte("same content")
	_, _, err := storeDocument(root, key, itemID, fileID, bytes.NewReader(content))
	require.NoError(t, err)
	path := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName, documentFilename(fileID))
	before, err := root.ReadFile(path)
	require.NoError(t, err)
	_, _, err = storeDocument(root, key, itemID, fileID, bytes.NewReader(content))
	require.Error(t, err)
	after, err := root.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Len(t, readDocumentEntries(t, root, filepath.Dir(path)), 1)
}

func TestDocumentStoreValidatesDirectoriesIDsAndKey(t *testing.T) {
	root, itemID, key := newDocumentStoreRoot(t)
	fileID := testDocumentID(t)
	for _, test := range []struct {
		name   string
		itemID string
		fileID string
		key    *paw.Key
	}{
		{name: "invalid item", itemID: "bad", fileID: fileID, key: key},
		{name: "invalid file", itemID: itemID, fileID: "bad", key: key},
		{name: "missing item", itemID: testDocumentID(t), fileID: fileID, key: key},
		{name: "nil key", itemID: itemID, fileID: fileID},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := storeDocument(root, test.key, test.itemID, test.fileID, bytes.NewReader(nil))
			require.Error(t, err)
		})
	}

	badRootPath := filepath.Join(t.TempDir(), "root")
	require.NoError(t, os.MkdirAll(filepath.Join(badRootPath, itemsDirectoryName), 0700))
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(badRootPath, itemsDirectoryName, filesystemID(itemID))))
	badRoot, err := os.OpenRoot(badRootPath)
	require.NoError(t, err)
	defer badRoot.Close()
	_, _, err = storeDocument(badRoot, key, itemID, fileID, bytes.NewReader(nil))
	require.Error(t, err)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)

	badDocsPath := filepath.Join(t.TempDir(), "root")
	itemPath := filepath.Join(badDocsPath, itemsDirectoryName, filesystemID(itemID))
	require.NoError(t, os.MkdirAll(itemPath, 0700))
	require.NoError(t, os.Symlink(outside, filepath.Join(itemPath, documentsDirectoryName)))
	badDocsRoot, err := os.OpenRoot(badDocsPath)
	require.NoError(t, err)
	defer badDocsRoot.Close()
	_, _, err = storeDocument(badDocsRoot, key, itemID, fileID, bytes.NewReader(nil))
	require.Error(t, err)
	entries, err = os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestDocumentStoreCleansUpAfterReadErrorAndLateCollision(t *testing.T) {
	root, itemID, key := newDocumentStoreRoot(t)
	fileID := testDocumentID(t)
	documents := filepath.Join(itemsDirectoryName, filesystemID(itemID), documentsDirectoryName)
	readErr := errors.New("synthetic source error")
	_, _, err := storeDocument(root, key, itemID, fileID, &errorReader{err: readErr})
	require.ErrorIs(t, err, readErr)
	entries := readDocumentEntries(t, root, documents)
	require.Empty(t, entries)

	fileID = testDocumentID(t)
	finalName := filepath.Join(documents, documentFilename(fileID))
	reader := &createSentinelReader{root: root, name: finalName}
	_, _, err = storeDocument(root, key, itemID, fileID, reader)
	require.Error(t, err)
	sentinel, err := root.ReadFile(finalName)
	require.NoError(t, err)
	require.Equal(t, []byte("sentinel"), sentinel)
	entries = readDocumentEntries(t, root, documents)
	require.Len(t, entries, 1)
}

func readDocumentEntries(t *testing.T, root *os.Root, path string) []os.DirEntry {
	t.Helper()
	directory, err := root.Open(path)
	require.NoError(t, err)
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	require.NoError(t, err)
	return entries
}

func newDocumentStoreRoot(t *testing.T) (*os.Root, string, *paw.Key) {
	t.Helper()
	directory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(directory, itemsDirectoryName), 0700))
	itemID := testDocumentID(t)
	require.NoError(t, os.Mkdir(filepath.Join(directory, itemsDirectoryName, filesystemID(itemID)), 0700))
	root, err := os.OpenRoot(directory)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, root.Close()) })
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	return root, itemID, key
}

type createSentinelReader struct {
	root *os.Root
	name string
	done bool
}

func (reader *createSentinelReader) Read(p []byte) (int, error) {
	if !reader.done {
		reader.done = true
		if err := reader.root.WriteFile(reader.name, []byte("sentinel"), 0600); err != nil {
			return 0, err
		}
		p[0] = 'x'
		return 1, nil
	}
	return 0, io.EOF
}
