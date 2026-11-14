// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

func TestReceiveAttachmentPreservesIDAndCleansOnlyNewUnreferencedFiles(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Attachment", "content")
	require.NoError(t, err)
	fileID, err := model.NewFileID()
	require.NoError(t, err)
	content := []byte("attachment data")
	digest := sha256.Sum256(content)
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "attachment",
		DecryptedSize: uint64(len(content)),
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
	created, err := store.ReceiveAttachment(vault, item.ID, file, bytes.NewReader(content))
	require.NoError(t, err)
	require.True(t, created)
	created, err = store.ReceiveAttachment(vault, item.ID, file, bytes.NewReader(nil))
	require.NoError(t, err)
	require.False(t, created)
	wrong := file
	wrong.DecryptedSize++
	_, err = store.ReceiveAttachment(vault, item.ID, wrong, bytes.NewReader(content))
	require.Error(t, err)
	require.NoError(t, item.AddFile(file))
	require.NoError(t, store.StoreItem(vault, item))
	require.NoError(t, store.CleanupAttachments(vault, item.ID, []string{file.ID}))
	var output bytes.Buffer
	require.NoError(t, store.WriteAttachment(vault, item.ID, file, &output))
	require.Equal(t, content, output.Bytes())
}

func TestReceiveAttachmentStopsAtDeclaredSize(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Attachment", "content")
	require.NoError(t, err)
	fileID, err := model.NewFileID()
	require.NoError(t, err)
	content := []byte("12345")
	digest := sha256.Sum256(content)
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "attachment",
		DecryptedSize: uint64(len(content)),
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
	// The extra byte exceeds the declared size; the trailing reader must never be read.
	oversized := io.MultiReader(
		bytes.NewReader(append(append([]byte{}, content...), 'x')),
		iotest.ErrReader(errors.New("read past declared size")),
	)
	created, err := store.ReceiveAttachment(vault, item.ID, file, oversized)
	require.False(t, created)
	require.ErrorContains(t, err, "does not match metadata")
	require.NotContains(t, err.Error(), "read past declared size")
}

func TestReceiveAttachmentCleansUpAfterInterruptedRead(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Interrupted attachment", "content")
	require.NoError(t, err)
	fileID, err := model.NewFileID()
	require.NoError(t, err)
	content := []byte("expected attachment")
	digest := sha256.Sum256(content)
	file := cxf.File{
		Type:          cxf.CredentialTypeFile,
		ID:            fileID,
		Name:          "attachment",
		DecryptedSize: uint64(len(content)),
		IntegrityHash: base64.RawURLEncoding.EncodeToString(digest[:]),
	}
	interrupted := io.MultiReader(bytes.NewReader(content[:4]), iotest.ErrReader(errors.New("connection interrupted")))
	created, err := store.ReceiveAttachment(vault, item.ID, file, interrupted)
	require.False(t, created)
	require.ErrorContains(t, err, "connection interrupted")
	valid, err := store.VerifyAttachment(vault, item.ID, file)
	require.NoError(t, err)
	require.False(t, valid)
}

func TestAttachmentTransferRequiresUnlockedVault(t *testing.T) {
	store := newTestFileStorage(t)
	itemID := testAttachmentID(t)
	file := cxf.File{Type: cxf.CredentialTypeFile, ID: testAttachmentID(t), Name: "attachment"}
	locked := &model.Vault{}
	t.Run("write", func(t *testing.T) {
		require.ErrorContains(t, store.WriteAttachment(locked, itemID, file, io.Discard), "unlocked vault")
	})
	t.Run("verify", func(t *testing.T) {
		_, err := store.VerifyAttachment(locked, itemID, file)
		require.ErrorContains(t, err, "unlocked vault")
	})
	t.Run("receive", func(t *testing.T) {
		_, err := store.ReceiveAttachment(locked, itemID, file, bytes.NewReader(nil))
		require.ErrorContains(t, err, "unlocked vault")
	})
	t.Run("cleanup", func(t *testing.T) {
		require.ErrorContains(t, store.CleanupAttachments(locked, itemID, nil), "unlocked vault")
	})
}
