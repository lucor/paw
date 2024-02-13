// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

func NewSSHKey() *SSHKey {
	now := time.Now()
	return &SSHKey{
		Metadata: &Metadata{
			Type:     SSHKeyItemType,
			Created:  now,
			Modified: now,
		},
		Passphrase: &Password{},
		Note:       &Note{},
	}
}
