// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package paw

import (
	"encoding/json"
	"net/url"
	"time"

	"golang.org/x/net/publicsuffix"
)

// Declare conformity to Item interface
var _ Item = (*Login)(nil)
var _ MetadataSubtitler = (*Login)(nil)

type Login struct {
	*Password `json:"password,omitempty"`
	*TOTP     `json:"totp,omitempty"`
	*Note     `json:"note,omitempty"`
	*Metadata `json:"metadata,omitempty"`

	Username string    `json:"username,omitempty"`
	URL      *LoginURL `json:"url,omitempty"`
}

// Subtitle implements MetadataSubtitler.
func (l *Login) Subtitle() string {
	return l.Username
}

func NewLogin() *Login {
	now := time.Now().UTC()
	return &Login{
		Metadata: &Metadata{
			Type:     LoginItemType,
			Created:  now,
			Modified: now,
			Autofill: &Autofill{},
		},
		Note:     NewNote(),
		Password: NewPassword(),
		TOTP:     NewDefaultTOTP(),
		URL:      NewLoginURL(),
	}
}

func NewLoginURL() *LoginURL {
	return &LoginURL{
		url: &url.URL{},
	}
}

type LoginURL struct {
	url        *url.URL `json:"-"`
	tldPlusOne string   `json:"-"`
}

func (u *LoginURL) URL() *url.URL {
	return u.url
}

func (u *LoginURL) TLDPlusOne() string {
	return u.tldPlusOne
}

func (u *LoginURL) String() string {
	if u.url == nil {
		return ""
	}
	return u.url.String()
}

func (u *LoginURL) Set(rawURL string) error {
	v, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	if v.String() == "" {
		u.url = v
		return nil
	}
	if v.Scheme == "" {
		// No scheme provided, default to https and parse again before return an error
		rawURL = "https://" + rawURL
		v, err = url.Parse(rawURL)
		if err != nil {
			return err
		}
	}
	u.url = v
	hostname := v.Hostname()
	tldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(hostname)
	if err != nil {
		u.tldPlusOne = hostname
	}
	u.tldPlusOne = tldPlusOne
	return nil
}

func (u *LoginURL) MarshalJSON() ([]byte, error) {
	if u.url == nil {
		return json.Marshal("")
	}
	rawURL := u.url.String()
	return json.Marshal(rawURL)
}

func (u *LoginURL) UnmarshalJSON(data []byte) error {
	var rawURL string
	err := json.Unmarshal(data, &rawURL)
	if err != nil {
		return err
	}
	if rawURL == "" {
		u.url = &url.URL{}
		return nil
	}
	return u.Set(rawURL)
}
