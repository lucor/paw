// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"archive/tar"
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"filippo.io/age"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
)

// PlaintextExportWarning describes the disclosure inherent in a plaintext export.
const PlaintextExportWarning = "The exported file will contain your credentials in plaintext. Anyone with access to this file can read them."

// WriteCXF writes a complete CXF document without changing the source vault.
func WriteCXF(ctx context.Context, w io.Writer, s *FileStorage, vault *model.Vault) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if vault == nil {
		return fmt.Errorf("cannot export: no vault selected")
	}
	if err := rejectAttachmentExport(ctx, s, vault); err != nil {
		return err
	}
	document, err := exportDocument(ctx, s, vault)
	if err != nil {
		return err
	}
	if err := cxf.Encode(exportWriter{ctx, w}, document); err != nil {
		return fmt.Errorf("write CXF: %w", err)
	}
	return ctx.Err()
}

func exportDocument(ctx context.Context, s *FileStorage, vault *model.Vault) (*cxf.Header, error) {
	// Copy before marshaling: Paw's marshaler updates the vault extension slice.
	data, err := json.Marshal(vault.Header, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("copy export document: %w", err)
	}
	copy, err := model.DecodeVault(data, vault.Key())
	if err != nil {
		return nil, fmt.Errorf("copy export vault: %w", err)
	}
	copy.Metadata = vault.Metadata
	data, err = json.Marshal(copy, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("encode vault metadata: %w", err)
	}
	var document cxf.Header
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	for a := range document.Accounts {
		for i, summary := range document.Accounts[a].Items {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			item, err := s.LoadItem(vault, summary.ID)
			if err != nil {
				return nil, fmt.Errorf("load export item: %w", err)
			}
			data, err := json.Marshal(item, json.Deterministic(true))
			if err != nil {
				return nil, fmt.Errorf("encode export item: %w", err)
			}
			if err := json.Unmarshal(data, &document.Accounts[a].Items[i]); err != nil {
				return nil, err
			}
		}
	}
	return &document, ctx.Err()
}

// WritePawArchive streams an encrypted TAR containing CXF and all referenced attachments.
func WritePawArchive(ctx context.Context, w io.Writer, s *FileStorage, vault *model.Vault, password string) error {
	if password == "" {
		return fmt.Errorf("archive password is required")
	}
	if vault == nil || len(vault.Accounts) == 0 {
		return fmt.Errorf("cannot export: no vault selected")
	}
	filesByItem := make(map[string][]cxf.File)
	for _, summary := range vault.Accounts[0].Items {
		item, err := s.LoadItem(vault, summary.ID)
		if err != nil {
			return err
		}
		files, err := itemAttachmentReferences(item)
		if err != nil {
			return err
		}
		filesByItem[item.ID] = files
	}
	document, err := exportDocument(ctx, s, vault)
	if err != nil {
		return err
	}
	var cxfData bytes.Buffer
	if err := cxf.Encode(exportWriter{ctx, &cxfData}, document); err != nil {
		return fmt.Errorf("encode archive CXF: %w", err)
	}
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return err
	}
	encrypted, err := age.Encrypt(exportWriter{ctx, w}, recipient)
	if err != nil {
		return err
	}
	tarWriter := tar.NewWriter(encrypted)
	if err := tarWriter.WriteHeader(&tar.Header{Name: "export.cxf", Mode: 0600, Size: int64(cxfData.Len()), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := io.Copy(tarWriter, &cxfData); err != nil {
		return err
	}
	root, err := s.openVaultRoot(vault.Accounts[0].ID)
	if err != nil {
		return err
	}
	defer root.Close()
	for itemID, files := range filesByItem {
		for _, file := range files {
			if err := writePawAttachment(ctx, tarWriter, root, vault, itemID, file); err != nil {
				return err
			}
		}
	}
	if err := tarWriter.Close(); err != nil {
		return err
	}
	return encrypted.Close()
}

func writePawAttachment(ctx context.Context, tw *tar.Writer, root *os.Root, vault *model.Vault, itemID string, file cxf.File) error {
	files, err := openAttachmentDirectory(root, itemID)
	if err != nil {
		return err
	}
	defer files.Close()
	source, err := files.Open(attachmentFilename(file.ID))
	if err != nil {
		return err
	}
	defer source.Close()
	name := filesDirectoryName + "/" + itemID + "/" + file.ID
	if file.DecryptedSize > uint64(^uint64(0)>>1) {
		return errors.New("attachment is too large for TAR")
	}
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(file.DecryptedSize), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	return decryptAttachment(vault.Key(), exportWriter{ctx: ctx, writer: tw}, source, file)
}

func itemAttachmentReferences(item *model.Item) ([]cxf.File, error) {
	files, err := item.Files()
	if err != nil {
		return nil, err
	}
	history, err := item.History()
	if err != nil {
		return nil, err
	}
	seen := make(map[string]cxf.File, len(files))
	for _, file := range files {
		seen[file.ID] = file
	}
	for _, entry := range history.Entries {
		data, err := json.Marshal(entry.Item, json.Deterministic(true))
		if err != nil {
			return nil, err
		}
		var snapshot model.Item
		if err := json.Unmarshal(data, &snapshot); err != nil {
			return nil, err
		}
		archived, err := snapshot.Files()
		if err != nil {
			return nil, err
		}
		for _, file := range archived {
			if current, ok := seen[file.ID]; ok {
				if current != file {
					return nil, fmt.Errorf("attachment %s has inconsistent history metadata", file.ID)
				}
				continue
			}
			seen[file.ID] = file
			files = append(files, file)
		}
	}
	return files, nil
}

// WriteEncryptedCXF streams CXF into a standard password-encrypted age file.
func WriteEncryptedCXF(ctx context.Context, w io.Writer, s *FileStorage, vault *model.Vault, password string) error {
	if password == "" {
		return fmt.Errorf("export password is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if vault == nil {
		return fmt.Errorf("cannot export: no vault selected")
	}
	if err := rejectAttachmentExport(ctx, s, vault); err != nil {
		return err
	}
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return fmt.Errorf("prepare export encryption: %w", err)
	}
	encrypted, err := age.Encrypt(exportWriter{ctx, w}, recipient)
	if err != nil {
		return fmt.Errorf("start export encryption: %w", err)
	}
	if err := WriteCXF(ctx, encrypted, s, vault); err != nil {
		return err
	}
	if err := encrypted.Close(); err != nil {
		return fmt.Errorf("finalize export encryption: %w", err)
	}
	return ctx.Err()
}

// CheckExportSupported reports whether a CXF export can represent the selected vault.
func CheckExportSupported(ctx context.Context, s *FileStorage, vault *model.Vault) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if vault == nil {
		return fmt.Errorf("cannot export: no vault selected")
	}
	return rejectAttachmentExport(ctx, s, vault)
}

// rejectAttachmentExport prevents CXF exports from silently omitting attachment blobs.
func rejectAttachmentExport(ctx context.Context, storage *FileStorage, vault *model.Vault) error {
	for _, summary := range vault.Accounts[0].Items {
		if err := ctx.Err(); err != nil {
			return err
		}
		item, err := storage.LoadItem(vault, summary.ID)
		if err != nil {
			return fmt.Errorf("cannot inspect export Item: %w", err)
		}
		files, err := item.Files()
		if err != nil {
			return fmt.Errorf("cannot inspect export attachment metadata: %w", err)
		}
		if len(files) != 0 {
			return fmt.Errorf("CXF export cannot include attachment files")
		}
		history, err := item.History()
		if err != nil {
			return fmt.Errorf("cannot inspect export Item history: %w", err)
		}
		for _, entry := range history.Entries {
			data, err := json.Marshal(entry.Item)
			if err != nil {
				return fmt.Errorf("cannot inspect export Item history: %w", err)
			}
			var snapshot model.Item
			if err := json.Unmarshal(data, &snapshot); err != nil {
				return fmt.Errorf("cannot inspect export Item history: %w", err)
			}
			files, err := snapshot.Files()
			if err != nil {
				return fmt.Errorf("cannot inspect export attachment history: %w", err)
			}
			if len(files) != 0 {
				return fmt.Errorf("CXF export cannot include attachment files")
			}
		}
	}
	return nil
}

// WriteExport writes the selected CXF export format to the supplied stream.
func WriteExport(ctx context.Context, w io.Writer, s *FileStorage, vault *model.Vault, password string, plaintext bool) error {
	if plaintext {
		return WriteCXF(ctx, w, s, vault)
	}
	return WriteEncryptedCXF(ctx, w, s, vault, password)
}

// ExportFile writes a private file and publishes it atomically without replacing existing files.
// Callers must obtain explicit consent before selecting plaintext.
func ExportFile(ctx context.Context, s *FileStorage, vault *model.Vault, path, password string, plaintext bool) (err error) {
	if !plaintext && password == "" {
		return fmt.Errorf("export password is required")
	}
	return publishExportFile(ctx, path, func(w io.Writer) error {
		return WriteExport(ctx, w, s, vault, password, plaintext)
	})
}

// ExportPawFile writes a password-encrypted Paw archive and publishes it atomically.
func ExportPawFile(ctx context.Context, s *FileStorage, vault *model.Vault, path, password string) error {
	if password == "" {
		return fmt.Errorf("archive password is required")
	}
	return publishExportFile(ctx, path, func(w io.Writer) error {
		return WritePawArchive(ctx, w, s, vault, password)
	})
}

func publishExportFile(ctx context.Context, path string, write func(io.Writer) error) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("export destination already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect export destination: %w", err)
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".paw-export-*")
	if err != nil {
		return fmt.Errorf("create export file: %w", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err := write(f); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync export file: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close export file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Link(f.Name(), path); err != nil {
		return fmt.Errorf("publish export file: %w", err)
	}
	return nil
}

type exportWriter struct {
	ctx    context.Context
	writer io.Writer
}

// Write checks for cancellation before writing export data.
func (w exportWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.writer.Write(p)
}
