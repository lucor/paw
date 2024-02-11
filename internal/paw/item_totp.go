// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

import (
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"

	"lucor.dev/paw/internal/otp"
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
)

type TOTP struct {
	Digits   int      `json:"digits,omitempty"`
	Hash     TOTPHash `json:"hash,omitempty"`
	Interval int      `json:"interval,omitempty"`
	Secret   string   `json:"secret,omitempty"`
}

// Hasher returns the hash function for the TOTP
func (t *TOTP) Hasher() func() hash.Hash {
	switch t.Hash {
	case SHA256:
		return sha256.New
	case SHA512:
		return sha512.New
	default:
		return sha1.New
	}
}

func NewDefaultTOTP() *TOTP {
	return &TOTP{
		Digits:   TOTPDigitsDefault,
		Hash:     TOTPHashDefault,
		Interval: TOTPIntervalDefault,
	}
}
