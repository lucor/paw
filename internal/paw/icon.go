// SPDX-FileCopyrightText: 2022-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

type FaviconFormat string

func (f FaviconFormat) String() string {
	return string(f)
}

func NewFavicon(host string, data []byte, format string) *Favicon {
	return &Favicon{
		Host:   host,
		Data:   data,
		Format: format,
	}
}

// Favicon represents a login favicon and it is a bundled fyne.resource compiled
// into the application
type Favicon struct {
	Host   string `json:"host,omitempty"`
	Data   []byte `json:"data,omitempty"`
	Format string `json:"format,omitempty"`
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
