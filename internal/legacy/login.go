// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"time"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// Login is a login decoded from the legacy Paw storage format.
type Login struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`
	Username  string        `json:"username,omitempty"`
	URL       *paw.LoginURL `json:"url,omitempty"`
}

// NewLogin creates a legacy login fixture with current metadata.
func NewLogin() *Login {
	now := time.Now().UTC()
	return &Login{
		Metadata: &Metadata{
			Type: model.LoginItemType, Created: now, Modified: now, Autofill: &model.Autofill{},
		},
		Note: NewNote(), Password: NewPassword(), TOTP: NewDefaultTOTP(), URL: paw.NewLoginURL(),
	}
}
