// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"

	"filippo.io/age"
	"go.lucor.dev/paw/internal/paw"
)

// ErrIncorrectMasterPassword identifies a failed current-password authentication.
var ErrIncorrectMasterPassword = errors.New("current master password is incorrect")

// ChangeMasterPassword atomically replaces the password protection of the existing key.
func (s *FileStorage) ChangeMasterPassword(ctx context.Context, id, current, next string) error {
	if !isVaultID(id) {
		return fmt.Errorf("invalid vault ID")
	}
	if current == "" || next == "" {
		return fmt.Errorf("master passwords are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := s.openVaultRoot(id)
	if err != nil {
		return fmt.Errorf("open protected key directory: %w", err)
	}
	defer root.Close()
	reader, err := root.Open(keyFileName)
	if err != nil {
		return fmt.Errorf("read protected vault key: %w", err)
	}
	key, readErr := paw.LoadKey(current, reader)
	closeErr := reader.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		var mismatch *age.NoIdentityMatchError
		if errors.As(err, &mismatch) {
			return ErrIncorrectMasterPassword
		}
		return fmt.Errorf("read protected vault key: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := atomicWrite(root, keyFileName, func(w io.Writer) error {
		if err := key.WriteWrapped(w, next); err != nil {
			return err
		}
		return ctx.Err()
	}); err != nil {
		return fmt.Errorf("replace protected key file: %w", err)
	}
	return nil
}
