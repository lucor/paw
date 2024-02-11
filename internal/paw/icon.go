// Copyright 2022 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

func NewFavicon(host string, data []byte) *Favicon {
	return &Favicon{
		Host: host,
		Data: data,
	}
}

// Favicon represents a login favicon and it is a bundled fyne.resource compiled
// into the application
type Favicon struct {
	Host string `json:"host,omitempty"`
	Data []byte `json:"data,omitempty"`
}

// Name returns the unique name of this resource, usually matching the host name it
// was downloaded from.
func (f *Favicon) Name() string {
	return f.Host
}

// Content returns the bytes of the favicon resource encoded as PNG
func (f *Favicon) Content() []byte {
	return f.Data
}
