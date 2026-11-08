// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build windows

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// atomicWrite stages beside the destination and propagates rename failures without deleting it.
func atomicWrite(root *os.Root, name string, write func(io.Writer) error) error {
	dir := filepath.Dir(name)
	var tempName string
	var temp *os.File
	for range 10 {
		var token [12]byte
		if _, err := rand.Read(token[:]); err != nil {
			return err
		}
		tempName = filepath.Join(dir, ".paw-"+hex.EncodeToString(token[:]))
		var err error
		temp, err = root.OpenFile(tempName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if !os.IsExist(err) {
			if err != nil {
				return err
			}
			break
		}
	}
	if temp == nil {
		return os.ErrExist
	}
	cleanup := func() error { return root.Remove(tempName) }
	if err := write(temp); err != nil {
		return errors.Join(err, temp.Close(), cleanup())
	}
	if err := temp.Sync(); err != nil {
		return errors.Join(err, temp.Close(), cleanup())
	}
	if err := temp.Close(); err != nil {
		return errors.Join(err, cleanup())
	}
	if err := root.Rename(tempName, name); err != nil {
		return errors.Join(err, cleanup())
	}
	return nil
}

// syncDirectory is unavailable through Go's portable Windows directory API.
func syncDirectory(root *os.Root, name string) error { return nil }
