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
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestAttachmentStorePublishesEncryptedFiles(t *testing.T) {
	require.Equal(t, "files", filesDirectoryName)
	for _, content := range [][]byte{[]byte("synthetic attachment"), {}, {0, 255, 1, 0, 42}} {
		t.Run(string(rune(len(content)+'0')), func(t *testing.T) {
			root, itemID, key := newAttachmentStoreRoot(t)
			fileID := testAttachmentID(t)
			size, digest, err := storeAttachment(root, key, itemID, fileID, bytes.NewReader(content))
			require.NoError(t, err)
			require.Equal(t, uint64(len(content)), size)
			path := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName, attachmentFilename(fileID))
			info, err := root.Stat(path)
			require.NoError(t, err)
			require.True(t, info.Mode().IsRegular())
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0 {
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
			require.NoError(t, decryptAttachment(key, &plaintext, ciphertext, file))
			require.True(t, bytes.Equal(content, plaintext.Bytes()))
			require.NoFileExists(t, filepath.Join(itemsDirectoryName, filesystemID(itemID), "item.age"))
			require.NoFileExists(t, vaultFileName)
			entries := readAttachmentEntries(t, root, filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName))
			require.Len(t, entries, 1)
			filesInfo, err := root.Stat(filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName))
			require.NoError(t, err)
			if runtime.GOOS != "windows" && filesInfo.Mode().Perm() != 0 {
				require.Equal(t, os.FileMode(0700), filesInfo.Mode().Perm())
			}
		})
	}
}

func TestAttachmentCleanupPreservesAmbiguousEntriesAndIsIdempotent(t *testing.T) {
	root, itemID, _ := newAttachmentStoreRoot(t)
	item, err := model.NewNote("cleanup", "")
	require.NoError(t, err)
	item.ID = itemID
	dir := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	require.NoError(t, root.Mkdir(dir, 0700))
	orphan := testAttachmentID(t)
	require.NoError(t, root.WriteFile(filepath.Join(dir, attachmentFilename(orphan)), []byte("orphan"), 0600))
	require.NoError(t, root.WriteFile(filepath.Join(dir, "unknown!.age"), []byte("keep"), 0600))
	require.NoError(t, root.WriteFile(filepath.Join(dir, "not-canonical!.age"), []byte("keep"), 0600))
	linkID := testAttachmentID(t)
	if err := root.Symlink("unknown!.age", filepath.Join(dir, attachmentFilename(linkID))); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	dirID := testAttachmentID(t)
	require.NoError(t, root.Mkdir(filepath.Join(dir, attachmentFilename(dirID)), 0700))
	storage := &FileStorage{}
	storage.cleanupItemAttachmentsBestEffort(root, item)
	_, err = root.Lstat(filepath.Join(dir, attachmentFilename(orphan)))
	require.ErrorIs(t, err, os.ErrNotExist)
	for _, name := range []string{"unknown!.age", "not-canonical!.age"} {
		got, err := root.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		require.Equal(t, []byte("keep"), got)
	}
	info, err := root.Lstat(filepath.Join(dir, attachmentFilename(linkID)))
	require.NoError(t, err)
	require.True(t, info.Mode()&os.ModeSymlink != 0)
	info, err = root.Stat(filepath.Join(dir, attachmentFilename(dirID)))
	require.NoError(t, err)
	require.True(t, info.IsDir())
	storage.cleanupItemAttachmentsBestEffort(root, item)
	_, err = root.Lstat(filepath.Join(dir, attachmentFilename(orphan)))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestAttachmentCleanupPreservesFilesForMalformedHistory(t *testing.T) {
	root, itemID, _ := newAttachmentStoreRoot(t)
	item, err := model.NewNote("cleanup", "")
	require.NoError(t, err)
	item.ID = itemID
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"paw.pm/history","schemaVersion":1,"entries":"invalid"}`))
	dir := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	require.NoError(t, root.Mkdir(dir, 0700))
	orphan := filepath.Join(dir, attachmentFilename(testAttachmentID(t)))
	require.NoError(t, root.WriteFile(orphan, []byte("preserve"), 0600))
	(&FileStorage{}).cleanupItemAttachmentsBestEffort(root, item)
	got, err := root.ReadFile(orphan)
	require.NoError(t, err)
	require.Equal(t, []byte("preserve"), got)
}

func TestAttachmentCleanupRemovesOnlyRecognizedStagingFilesAndClosesDirectory(t *testing.T) {
	root, itemID, _ := newAttachmentStoreRoot(t)
	item, err := model.NewNote("cleanup", "")
	require.NoError(t, err)
	item.ID = itemID
	filesDir := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	require.NoError(t, root.Mkdir(filesDir, 0700))
	staged := filepath.Join(filesDir, ".paw-attachment-0123456789abcdef01234567.tmp")
	require.NoError(t, root.WriteFile(staged, []byte("encrypted staging data"), 0600))
	unknown := filepath.Join(filesDir, ".paw-attachment-not-a-token.tmp")
	require.NoError(t, root.WriteFile(unknown, []byte("preserve"), 0600))
	uppercase := filepath.Join(filesDir, ".paw-attachment-ABCDEF0123456789ABCDEF01.tmp")
	require.NoError(t, root.WriteFile(uppercase, []byte("preserve"), 0600))

	(&FileStorage{}).cleanupItemAttachmentsBestEffort(root, item)
	_, err = root.Lstat(staged)
	require.ErrorIs(t, err, os.ErrNotExist)
	got, err := root.ReadFile(unknown)
	require.NoError(t, err)
	require.Equal(t, []byte("preserve"), got)
	got, err = root.ReadFile(uppercase)
	require.NoError(t, err)
	require.Equal(t, []byte("preserve"), got)
	require.NoError(t, root.Remove(unknown))
	require.NoError(t, root.Remove(uppercase))
	require.NoError(t, root.Remove(filesDir), "cleanup must close its directory handle")
}

func TestAttachmentStoreRejectsExistingTargets(t *testing.T) {
	for _, kind := range []string{"regular", "directory", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			root, itemID, key := newAttachmentStoreRoot(t)
			fileID := testAttachmentID(t)
			filesDir := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
			require.NoError(t, root.Mkdir(filesDir, 0700))
			name := attachmentFilename(fileID)
			switch kind {
			case "regular":
				require.NoError(t, root.WriteFile(filepath.Join(filesDir, name), []byte("sentinel"), 0600))
			case "directory":
				require.NoError(t, root.Mkdir(filepath.Join(filesDir, name), 0700))
			case "symlink":
				outside := filepath.Join(t.TempDir(), "sentinel")
				require.NoError(t, os.WriteFile(outside, []byte("sentinel"), 0600))
				if err := root.Symlink(outside, filepath.Join(filesDir, name)); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			_, _, err := storeAttachment(root, key, itemID, fileID, bytes.NewReader([]byte("content")))
			require.Error(t, err)
			entries := readAttachmentEntries(t, root, filesDir)
			require.Len(t, entries, 1)
			if kind == "regular" {
				got, err := root.ReadFile(filepath.Join(filesDir, name))
				require.NoError(t, err)
				require.Equal(t, []byte("sentinel"), got)
			}
		})
	}
}

func TestAttachmentStoreRejectsSameContentCollision(t *testing.T) {
	root, itemID, key := newAttachmentStoreRoot(t)
	fileID := testAttachmentID(t)
	content := []byte("same content")
	_, _, err := storeAttachment(root, key, itemID, fileID, bytes.NewReader(content))
	require.NoError(t, err)
	path := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName, attachmentFilename(fileID))
	before, err := root.ReadFile(path)
	require.NoError(t, err)
	_, _, err = storeAttachment(root, key, itemID, fileID, bytes.NewReader(content))
	require.Error(t, err)
	after, err := root.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Len(t, readAttachmentEntries(t, root, filepath.Dir(path)), 1)
}

func TestAttachmentStoreValidatesDirectoriesIDsAndKey(t *testing.T) {
	root, itemID, key := newAttachmentStoreRoot(t)
	fileID := testAttachmentID(t)
	for _, test := range []struct {
		name   string
		itemID string
		fileID string
		key    *paw.Key
	}{
		{name: "invalid item", itemID: "bad", fileID: fileID, key: key},
		{name: "invalid file", itemID: itemID, fileID: "bad", key: key},
		{name: "missing item", itemID: testAttachmentID(t), fileID: fileID, key: key},
		{name: "nil key", itemID: itemID, fileID: fileID},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := storeAttachment(root, test.key, test.itemID, test.fileID, bytes.NewReader(nil))
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
	_, _, err = storeAttachment(badRoot, key, itemID, fileID, bytes.NewReader(nil))
	require.Error(t, err)
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)

	badFilesPath := filepath.Join(t.TempDir(), "root")
	itemPath := filepath.Join(badFilesPath, itemsDirectoryName, filesystemID(itemID))
	require.NoError(t, os.MkdirAll(itemPath, 0700))
	require.NoError(t, os.Symlink(outside, filepath.Join(itemPath, filesDirectoryName)))
	badFilesRoot, err := os.OpenRoot(badFilesPath)
	require.NoError(t, err)
	defer badFilesRoot.Close()
	_, _, err = storeAttachment(badFilesRoot, key, itemID, fileID, bytes.NewReader(nil))
	require.Error(t, err)
	entries, err = os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestAttachmentStoreCleansUpAfterReadErrorAndLateCollision(t *testing.T) {
	root, itemID, key := newAttachmentStoreRoot(t)
	fileID := testAttachmentID(t)
	filesDir := filepath.Join(itemsDirectoryName, filesystemID(itemID), filesDirectoryName)
	readErr := errors.New("synthetic source error")
	_, _, err := storeAttachment(root, key, itemID, fileID, &errorReader{err: readErr})
	require.ErrorIs(t, err, readErr)
	entries := readAttachmentEntries(t, root, filesDir)
	require.Empty(t, entries)

	fileID = testAttachmentID(t)
	finalName := filepath.Join(filesDir, attachmentFilename(fileID))
	reader := &createSentinelReader{root: root, name: finalName}
	_, _, err = storeAttachment(root, key, itemID, fileID, reader)
	require.Error(t, err)
	sentinel, err := root.ReadFile(finalName)
	require.NoError(t, err)
	require.Equal(t, []byte("sentinel"), sentinel)
	entries = readAttachmentEntries(t, root, filesDir)
	require.Len(t, entries, 1)
}

func readAttachmentEntries(t *testing.T, root *os.Root, path string) []os.DirEntry {
	t.Helper()
	directory, err := root.Open(path)
	require.NoError(t, err)
	defer directory.Close()
	entries, err := directory.ReadDir(-1)
	require.NoError(t, err)
	return entries
}

func newAttachmentStoreRoot(t *testing.T) (*os.Root, string, *paw.Key) {
	t.Helper()
	directory := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(directory, itemsDirectoryName), 0700))
	itemID := testAttachmentID(t)
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
