// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/paw"
)

const documentStreamBufferSize = 32 * 1024

// encryptDocument streams plaintext into an age message and returns its size and SHA-256 digest.
func encryptDocument(key *paw.Key, dst io.Writer, src io.Reader) (uint64, string, error) {
	if key == nil {
		return 0, "", errors.New("vault key is missing")
	}
	encWriter, err := key.Encrypt(dst)
	if err != nil {
		return 0, "", fmt.Errorf("cannot start document encryption: %w", err)
	}
	digest := sha256.New()
	counter := &documentCounter{writer: io.MultiWriter(encWriter, digest)}
	transferErr := copyDocument(counter, src)
	closeErr := encWriter.Close()
	if transferErr != nil || closeErr != nil {
		return 0, "", errors.Join(
			wrapDocumentError("cannot encrypt document", transferErr),
			wrapDocumentError("cannot finish document encryption", closeErr),
		)
	}
	return counter.size, base64.RawURLEncoding.EncodeToString(digest.Sum(nil)), nil
}

// decryptDocument streams and authenticates an age message, then checks its CXF metadata.
func decryptDocument(key *paw.Key, dst io.Writer, src io.Reader, expected cxf.File) error {
	if key == nil {
		return errors.New("vault key is missing")
	}
	if err := expected.Validate(); err != nil {
		return fmt.Errorf("invalid document metadata: %w", err)
	}
	decrypted, err := key.Decrypt(src)
	if err != nil {
		return fmt.Errorf("cannot start document decryption: %w", err)
	}
	digest := sha256.New()
	counter := &documentCounter{writer: io.MultiWriter(dst, digest)}
	if err := copyDocument(counter, decrypted); err != nil {
		return fmt.Errorf("cannot decrypt document: %w", err)
	}
	actualHash := base64.RawURLEncoding.EncodeToString(digest.Sum(nil))
	if counter.size != expected.DecryptedSize {
		return fmt.Errorf("document size does not match metadata")
	}
	if actualHash != expected.IntegrityHash {
		return fmt.Errorf("document integrity does not match metadata")
	}
	return nil
}

// documentCounter writes data while tracking the number of bytes accepted by its destination.
type documentCounter struct {
	writer io.Writer
	size   uint64
}

// Write tracks successfully transferred bytes and rejects uint64 size overflow.
func (counter *documentCounter) Write(data []byte) (int, error) {
	if uint64(len(data)) > ^uint64(0)-counter.size {
		return 0, errors.New("document size exceeds supported range")
	}
	n, err := counter.writer.Write(data)
	counter.size += uint64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}

// copyDocument transfers a stream with a fixed-size buffer.
func copyDocument(dst io.Writer, src io.Reader) error {
	buffer := make([]byte, documentStreamBufferSize)
	// Hide optional WriterTo so io.CopyBuffer always uses the bounded buffer.
	_, err := io.CopyBuffer(dst, readerOnly{Reader: src}, buffer)
	return err
}

type readerOnly struct{ io.Reader }

// wrapDocumentError adds context only when an error is present.
func wrapDocumentError(message string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}
