// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	"go.lucor.dev/paw/internal/otp"
)

type TOTPHash string

const (
	SHA1   TOTPHash = "SHA1"
	SHA256 TOTPHash = "SHA256"
	SHA512 TOTPHash = "SHA512"
)

const (
	TOTPHashDefault     = SHA1
	TOTPDigitsDefault   = otp.DefaultDigits
	TOTPIntervalDefault = otp.DefaultInterval
	TOTPDigitsMinimum   = 5
	TOTPDigitsMaximum   = 10
)

// Hasher returns the hash function for an authenticator algorithm.
func (algorithm TOTPHash) Hasher() func() hash.Hash {
	switch algorithm {
	case SHA256:
		return sha256.New
	case SHA512:
		return sha512.New
	default:
		return sha1.New
	}
}
