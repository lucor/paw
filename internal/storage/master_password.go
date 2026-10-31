// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"filippo.io/age"
)

// ErrIncorrectMasterPassword identifies a failed current-password authentication.
var ErrIncorrectMasterPassword = errors.New("current master password is incorrect")

// ChangeMasterPassword atomically replaces the password protection of the existing key.
func (s *OSStorage) ChangeMasterPassword(ctx context.Context, id, current, next string) error {
	return changeMasterPassword(ctx, s, id, current, next)
}

// ChangeMasterPassword uses the app's local storage directory for atomic replacement.
func (s *FyneStorage) ChangeMasterPassword(ctx context.Context, id, current, next string) error {
	return changeMasterPassword(ctx, s, id, current, next)
}

// changeMasterPassword publishes only a complete encrypted key file in the same directory.
func changeMasterPassword(ctx context.Context, s Storage, id, current, next string) error {
	if !isVaultID(id) {
		return fmt.Errorf("invalid vault ID")
	}
	if current == "" || next == "" {
		return fmt.Errorf("master passwords are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := s.LoadVaultKey(id, current)
	if err != nil {
		var mismatch *age.NoIdentityMatchError
		if errors.As(err, &mismatch) {
			return ErrIncorrectMasterPassword
		}
		return fmt.Errorf("read protected vault key: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	path := keyPath(s, id)
	temp, err := os.CreateTemp(filepath.Dir(path), ".key-*")
	if err != nil {
		return fmt.Errorf("create protected key file: %w", err)
	}
	defer os.Remove(temp.Name())
	defer temp.Close()
	if err := key.WriteWrapped(temp, next); err != nil {
		return fmt.Errorf("protect vault key: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync protected key file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close protected key file: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("replace protected key file: %w", err)
	}
	return nil
}
