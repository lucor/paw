// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package legacy

import (
	"time"

	"go.lucor.dev/paw/internal/model"
)

// SSHKey is an SSH key decoded from the legacy Paw storage format.
type SSHKey struct {
	*Metadata   `json:"metadata,omitempty"`
	*Note       `json:"note,omitempty"`
	AddToAgent  bool      `json:"add_to_agent,omitempty"`
	Comment     string    `json:"comment,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Passphrase  *Password `json:"passphrase,omitempty"`
	PrivateKey  string    `json:"private_key,omitempty"`
	PublicKey   string    `json:"public_key,omitempty"`
}

// NewSSHKey creates a legacy SSH-key fixture with current metadata.
func NewSSHKey() *SSHKey {
	now := time.Now().UTC()
	return &SSHKey{
		Metadata:   &Metadata{Type: model.SSHKeyItemType, Created: now, Modified: now},
		Passphrase: NewPassword(), Note: NewNote(),
	}
}
