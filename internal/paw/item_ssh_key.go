// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package paw

import (
	"time"
)

// Declare conformity to Item interface
var _ Item = (*SSHKey)(nil)

type SSHKey struct {
	*Metadata `json:"metadata,omitempty"`
	*Note     `json:"note,omitempty"`

	AddToAgent  bool      `json:"add_to_agent,omitempty"`
	Comment     string    `json:"comment,omitempty"`
	Fingerprint string    `json:"fingerprint,omitempty"`
	Passphrase  *Password `json:"passphrase,omitempty"`
	PrivateKey  string    `json:"private_key,omitempty"`
	PublicKey   string    `json:"public_key,omitempty"`
}

// Subtitle implements MetadataSubtitler.
func (i *SSHKey) Subtitle() string {
	return i.Comment
}

func NewSSHKey() *SSHKey {
	now := time.Now().UTC()
	return &SSHKey{
		Metadata: &Metadata{
			Type:     SSHKeyItemType,
			Created:  now,
			Modified: now,
		},
		Passphrase: NewPassword(),
		Note:       NewNote(),
	}
}
