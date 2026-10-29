// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"encoding/json"
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
func WriteCXF(ctx context.Context, w io.Writer, s Storage, vault *model.Vault) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if vault == nil {
		return fmt.Errorf("cannot export: no vault selected")
	}
	// Copy before marshaling: Paw's marshaler updates the vault extension slice.
	data, err := json.Marshal(vault.Header)
	if err != nil {
		return fmt.Errorf("copy export document: %w", err)
	}
	copy, err := model.DecodeVault(data, vault.Key())
	if err != nil {
		return fmt.Errorf("copy export vault: %w", err)
	}
	copy.Metadata = vault.Metadata
	data, err = json.Marshal(copy)
	if err != nil {
		return fmt.Errorf("encode vault metadata: %w", err)
	}
	var document cxf.Header
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	for a := range document.Accounts {
		for i, summary := range document.Accounts[a].Items {
			if err := ctx.Err(); err != nil {
				return err
			}
			item, err := s.LoadItem(vault, summary.ID)
			if err != nil {
				return fmt.Errorf("load export item: %w", err)
			}
			data, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("encode export item: %w", err)
			}
			if err := json.Unmarshal(data, &document.Accounts[a].Items[i]); err != nil {
				return err
			}
		}
	}
	if err := cxf.Encode(exportWriter{ctx, w}, &document); err != nil {
		return fmt.Errorf("write CXF: %w", err)
	}
	return ctx.Err()
}

// WriteEncryptedCXF streams CXF into a standard password-encrypted age file.
func WriteEncryptedCXF(ctx context.Context, w io.Writer, s Storage, vault *model.Vault, password string) error {
	if password == "" {
		return fmt.Errorf("export password is required")
	}
	if err := ctx.Err(); err != nil {
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

// ExportFile writes a private file and publishes it atomically without replacing existing files.
// Callers must obtain explicit consent before selecting plaintext.
func ExportFile(ctx context.Context, s Storage, vault *model.Vault, path, password string, plaintext bool) (err error) {
	if !plaintext && password == "" {
		return fmt.Errorf("export password is required")
	}
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
	if plaintext {
		err = WriteCXF(ctx, f, s, vault)
	} else {
		err = WriteEncryptedCXF(ctx, f, s, vault, password)
	}
	if err != nil {
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

func (w exportWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.writer.Write(p)
}
