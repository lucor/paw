package paw

import (
	"time"
)

// Declare conformity to Item interface
var _ Item = (*SSHKey)(nil)

type SSHKey struct {
	*Metadata `json:"metadata,omitempty"`
	*Note     `json:"note,omitempty"`

	AddToAgent  bool   `json:"add_to_agent,omitempty"`
	Comment     string `json:"comment,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	PrivateKey  string `json:"private_key,omitempty"`
	PublicKey   string `json:"public_key,omitempty"`
}

func NewSSHKey() *SSHKey {
	now := time.Now()
	return &SSHKey{
		Metadata: &Metadata{
			Type:     SSHKeyItemType,
			Created:  now,
			Modified: now,
		},
		Note: &Note{},
	}
}
