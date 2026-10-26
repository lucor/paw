// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration

import (
	"fmt"

	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

// MigrateTOTP converts a configured legacy TOTP to its CXF credential.
func MigrateTOTP(old *paw.TOTP) (*cxf.TOTP, error) {
	if old == nil || old.Secret == "" {
		return nil, nil
	}

	var algorithm string
	switch old.Hash {
	case "", paw.SHA1:
		algorithm = cxf.OTPHashAlgorithmSHA1
	case paw.SHA256:
		algorithm = cxf.OTPHashAlgorithmSHA256
	case paw.SHA512:
		algorithm = cxf.OTPHashAlgorithmSHA512
	default:
		return nil, fmt.Errorf("cannot migrate TOTP: unsupported algorithm")
	}
	if old.Interval < 1 || old.Interval > 1<<16-1 {
		return nil, fmt.Errorf("cannot migrate TOTP: period is out of range")
	}
	if old.Digits < 1 || old.Digits > 1<<16-1 {
		return nil, fmt.Errorf("cannot migrate TOTP: digits are out of range")
	}

	credential := &cxf.TOTP{
		Type:      cxf.CredentialTypeTOTP,
		Secret:    old.Secret,
		Period:    uint16(old.Interval),
		Digits:    uint16(old.Digits),
		Algorithm: algorithm,
	}
	if err := credential.Validate(); err != nil {
		return nil, fmt.Errorf("cannot migrate TOTP: invalid credential")
	}
	return credential, nil
}
