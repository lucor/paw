// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

import (
	"time"
)

// Declare conformity to Item interface
var _ Item = (*Login)(nil)
var _ MetadataSubtitler = (*Login)(nil)

type Login struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`

	Username string `json:"username,omitempty"`
	URL      string `json:"url,omitempty"`
}

// Subtitle implements MetadataSubtitler.
func (l *Login) Subtitle() string {
	return l.Username
}

func NewLogin() *Login {
	now := time.Now()
	return &Login{
		Metadata: &Metadata{
			Type:     LoginItemType,
			Created:  now,
			Modified: now,
			Autofill: &Autofill{},
		},
		Note:     &Note{},
		Password: &Password{},
		TOTP:     &TOTP{},
	}
}
