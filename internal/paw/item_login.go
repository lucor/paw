package paw

import (
	"time"
)

// Declare conformity to Item interface
var _ Item = (*Login)(nil)

type Login struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`

	Username string `json:"username,omitempty"`
	URL      string `json:"url,omitempty"`
}

func NewLogin() *Login {
	now := time.Now()
	return &Login{
		Metadata: &Metadata{
			Type:     LoginItemType,
			Created:  now,
			Modified: now,
		},
		Note:     &Note{},
		Password: &Password{},
		TOTP:     &TOTP{},
	}
}
