// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	json "encoding/json/v2"
	"net/url"

	"golang.org/x/net/publicsuffix"
)

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
		return json.Marshal("", json.Deterministic(true))
	}
	rawURL := u.url.String()
	return json.Marshal(rawURL, json.Deterministic(true))
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
