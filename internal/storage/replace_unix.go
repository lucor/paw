// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !windows

package storage

import (
	"errors"
	"io"
	"os"

	"github.com/google/renameio/v2"
)

// atomicWrite stages beside the destination; rename and sync guarantees depend on the filesystem.
func atomicWrite(root *os.Root, name string, write func(io.Writer) error) error {
	pending, err := renameio.NewPendingFile(name, renameio.WithRoot(root), renameio.WithStaticPermissions(0600))
	if err != nil {
		return err
	}
	if err := write(pending); err != nil {
		return errors.Join(err, pending.Cleanup())
	}
	if err := pending.CloseAtomicallyReplace(); err != nil {
		return errors.Join(err, pending.Cleanup())
	}
	return nil
}
