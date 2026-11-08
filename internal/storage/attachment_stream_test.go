// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"hash"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestAttachmentStreamRoundTripsSmallEmptyAndBinary(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	for _, content := range [][]byte{[]byte("synthetic attachment"), {}, {0, 255, 1, 128, 0, 42}} {
		t.Run(string(rune(len(content)+'0')), func(t *testing.T) {
			var ciphertext bytes.Buffer
			size, digest, err := encryptAttachment(key, &ciphertext, bytes.NewReader(content))
			require.NoError(t, err)
			wantHash := sha256.Sum256(content)
			require.Equal(t, uint64(len(content)), size)
			require.Equal(t, base64.RawURLEncoding.EncodeToString(wantHash[:]), digest)
			file := cxf.File{Type: cxf.CredentialTypeFile, ID: testAttachmentID(t), Name: "synthetic.bin", DecryptedSize: size, IntegrityHash: digest}
			var plaintext bytes.Buffer
			require.NoError(t, decryptAttachment(key, &plaintext, bytes.NewReader(ciphertext.Bytes()), file))
			require.True(t, bytes.Equal(content, plaintext.Bytes()))
		})
	}
}

func TestAttachmentStreamTransfersGeneratedLargeContent(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	const size = 16 * 1024 * 1024
	reader := &patternReader{remaining: size}
	ciphertext := &observingWriter{beforeEOF: func() bool { return !reader.eof }, payloadThreshold: 256}
	actualSize, digest, err := encryptAttachment(key, ciphertext, reader)
	require.NoError(t, err)
	require.True(t, reader.maxRequest <= attachmentStreamBufferSize)
	require.True(t, ciphertext.wroteBeforeEOF)
	require.Equal(t, uint64(size), actualSize)

	file := cxf.File{Type: cxf.CredentialTypeFile, ID: testAttachmentID(t), Name: "large.bin", DecryptedSize: actualSize, IntegrityHash: digest}
	sink := &hashSink{hash: sha256.New()}
	require.NoError(t, decryptAttachment(key, sink, bytes.NewReader(ciphertext.data), file))
	require.Equal(t, uint64(size), sink.size)
	require.Equal(t, digest, base64.RawURLEncoding.EncodeToString(sink.hash.Sum(nil)))
}

func TestAttachmentStreamRejectsBadKeyCiphertextAndMetadata(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	otherKey, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	content := []byte("binary\x00content")
	var encrypted bytes.Buffer
	size, digest, err := encryptAttachment(key, &encrypted, bytes.NewReader(content))
	require.NoError(t, err)
	valid := cxf.File{Type: cxf.CredentialTypeFile, ID: testAttachmentID(t), Name: "file.bin", DecryptedSize: size, IntegrityHash: digest}

	for name, test := range map[string]struct {
		key  *paw.Key
		data []byte
		file cxf.File
	}{
		"wrong key":            {key: otherKey, data: encrypted.Bytes(), file: valid},
		"tampered ciphertext":  {key: key, data: tamperAttachment(encrypted.Bytes()), file: valid},
		"truncated ciphertext": {key: key, data: encrypted.Bytes()[:encrypted.Len()-8], file: valid},
		"size mismatch":        {key: key, data: encrypted.Bytes(), file: cxf.File{Type: valid.Type, ID: valid.ID, Name: valid.Name, DecryptedSize: size + 1, IntegrityHash: digest}},
		"digest mismatch":      {key: key, data: encrypted.Bytes(), file: cxf.File{Type: valid.Type, ID: valid.ID, Name: valid.Name, DecryptedSize: size, IntegrityHash: base64.RawURLEncoding.EncodeToString(make([]byte, sha256.Size))}},
		"malformed metadata":   {key: key, data: encrypted.Bytes(), file: cxf.File{}},
		"nil key":              {key: nil, data: encrypted.Bytes(), file: valid},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			err := decryptAttachment(test.key, &output, bytes.NewReader(test.data), test.file)
			require.Error(t, err)
		})
	}
	_, _, err = encryptAttachment(nil, &bytes.Buffer{}, bytes.NewReader(content))
	require.Error(t, err)
}

func TestAttachmentStreamPropagatesSourceAndDestinationErrors(t *testing.T) {
	key, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	sourceErr := errors.New("synthetic source failure")
	_, _, err = encryptAttachment(key, &bytes.Buffer{}, &errorReader{err: sourceErr})
	require.ErrorIs(t, err, sourceErr)

	var ciphertext bytes.Buffer
	content := []byte("some test content that exceeds the destination limit")
	size, digest, err := encryptAttachment(key, &ciphertext, bytes.NewReader(content))
	require.NoError(t, err)
	file := cxf.File{Type: cxf.CredentialTypeFile, ID: testAttachmentID(t), Name: "failure.bin", DecryptedSize: size, IntegrityHash: digest}
	destinationErr := errors.New("synthetic destination failure")
	_, _, err = encryptAttachment(key, &failWriter{remaining: 8, err: destinationErr}, bytes.NewReader(content))
	require.ErrorIs(t, err, destinationErr)
	err = decryptAttachment(key, &failWriter{remaining: 4, err: destinationErr}, bytes.NewReader(ciphertext.Bytes()), file)
	require.ErrorIs(t, err, destinationErr)
	err = decryptAttachment(key, &bytes.Buffer{}, &errorAfterReader{data: ciphertext.Bytes(), err: sourceErr}, file)
	require.ErrorIs(t, err, sourceErr)
	_, _, err = encryptAttachment(key, shortWriter{}, bytes.NewReader(content))
	require.ErrorIs(t, err, io.ErrShortWrite)
	err = decryptAttachment(key, shortWriter{}, bytes.NewReader(ciphertext.Bytes()), file)
	require.ErrorIs(t, err, io.ErrShortWrite)
	finalWriter := &failAfterWritesWriter{failAfter: 1, err: destinationErr}
	_, _, err = encryptAttachment(key, finalWriter, bytes.NewReader(nil))
	require.ErrorIs(t, err, destinationErr)
	require.True(t, finalWriter.failedAfterWrite)
}

type patternReader struct {
	remaining  int
	eof        bool
	maxRequest int
}

func (reader *patternReader) Read(p []byte) (int, error) {
	if len(p) > reader.maxRequest {
		reader.maxRequest = len(p)
	}
	if reader.remaining == 0 {
		reader.eof = true
		return 0, io.EOF
	}
	n := min(len(p), reader.remaining)
	for i := range p[:n] {
		p[i] = byte(i)
	}
	reader.remaining -= n
	return n, nil
}

type observingWriter struct {
	data             []byte
	wroteBeforeEOF   bool
	beforeEOF        func() bool
	payloadThreshold int
}

func (writer *observingWriter) Write(p []byte) (int, error) {
	if len(writer.data)+len(p) > writer.payloadThreshold && writer.beforeEOF() {
		writer.wroteBeforeEOF = true
	}
	writer.data = append(writer.data, p...)
	return len(p), nil
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) {
	return max(0, len(p)-1), nil
}

type failAfterWritesWriter struct {
	writes           int
	failAfter        int
	failedAfterWrite bool
	err              error
}

func (writer *failAfterWritesWriter) Write(p []byte) (int, error) {
	if writer.writes >= writer.failAfter {
		writer.failedAfterWrite = true
		return 0, writer.err
	}
	writer.writes++
	return len(p), nil
}

type errorAfterReader struct {
	data []byte
	err  error
}

func (reader *errorAfterReader) Read(p []byte) (int, error) {
	if len(reader.data) == 0 {
		return 0, reader.err
	}
	n := copy(p, reader.data)
	reader.data = reader.data[n:]
	return n, nil
}

type hashSink struct {
	hash hash.Hash
	size uint64
}

func (sink *hashSink) Write(p []byte) (int, error) {
	n, err := sink.hash.Write(p)
	sink.size += uint64(n)
	return n, err
}

type errorReader struct{ err error }

func (reader *errorReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	p[0] = 'x'
	return 1, reader.err
}

type failWriter struct {
	remaining int
	err       error
}

func (writer *failWriter) Write(p []byte) (int, error) {
	if writer.remaining == 0 {
		return 0, writer.err
	}
	n := min(len(p), writer.remaining)
	writer.remaining -= n
	if n < len(p) {
		return n, writer.err
	}
	return n, nil
}

func tamperAttachment(data []byte) []byte {
	copyData := bytes.Clone(data)
	copyData[len(copyData)-1] ^= 0x01
	return copyData
}

func testAttachmentID(t *testing.T) string {
	t.Helper()
	id, err := model.NewFileID()
	require.NoError(t, err)
	return id
}
