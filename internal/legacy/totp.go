// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import "go.lucor.dev/paw/internal/paw"

// TOTP stores authenticator settings in a legacy Paw login.
type TOTP struct {
	Digits   int          `json:"digits,omitempty"`
	Hash     paw.TOTPHash `json:"hash,omitempty"`
	Interval int          `json:"interval,omitempty"`
	Secret   string       `json:"secret,omitempty"`
}

// NewDefaultTOTP creates a legacy TOTP value with Paw's standard defaults.
func NewDefaultTOTP() *TOTP {
	return &TOTP{
		Digits: paw.TOTPDigitsDefault, Hash: paw.TOTPHashDefault, Interval: paw.TOTPIntervalDefault,
	}
}
