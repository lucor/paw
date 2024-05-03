// Copyright 2024 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package paw

func newDefaultPreferences() *Preferences {
	return &Preferences{
		FaviconDownloader: FaviconDownloaderPreferences{
			Disabled: false,
		},
		Password: PasswordPreferences{
			Passphrase: PassphrasePasswordPreferences{
				DefaultLength: PassphrasePasswordDefaultLength,
				MaxLength:     PassphrasePasswordMaxLength,
				MinLength:     PassphrasePasswordMinLength,
			},
			Pin: PinPasswordPreferences{
				DefaultLength: PinPasswordDefaultLength,
				MaxLength:     PinPasswordMaxLength,
				MinLength:     PinPasswordMinLength,
			},
			Random: RandomPasswordPreferences{
				DefaultLength: RandomPasswordDefaultLength,
				DefaultFormat: RandomPasswordDefaultFormat,
				MaxLength:     RandomPasswordMaxLength,
				MinLength:     RandomPasswordMinLength,
			},
		},
		TOTP: TOTPPreferences{
			Digits:   TOTPDigitsDefault,
			Hash:     TOTPHashDefault,
			Interval: TOTPIntervalDefault,
		},
	}
}

type Preferences struct {
	FaviconDownloader FaviconDownloaderPreferences `json:"favicon_downloader,omitempty"`
	Password          PasswordPreferences          `json:"password,omitempty"`
	TOTP              TOTPPreferences              `json:"totp,omitempty"`
}

// FaviconDownloaderPreferences represents the preferences for the favicon downloader.
// FaviconDownloader tool is opt-out, hence the default value is false.
type FaviconDownloaderPreferences struct {
	Disabled bool `json:"disabled,omitempty"` // Disabled is true if the favicon downloader is disabled.
}

type PasswordPreferences struct {
	Passphrase PassphrasePasswordPreferences `json:"passphrase,omitempty"`
	Pin        PinPasswordPreferences        `json:"pin,omitempty"`
	Random     RandomPasswordPreferences     `json:"random,omitempty"`
}

type PassphrasePasswordPreferences struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}

type PinPasswordPreferences struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}
type RandomPasswordPreferences struct {
	DefaultLength int    `json:"default_length,omitempty"`
	DefaultFormat Format `json:"default_format,omitempty"`
	MaxLength     int    `json:"max_length,omitempty"`
	MinLength     int    `json:"min_length,omitempty"`
}

type TOTPPreferences struct {
	Digits   int      `json:"digits,omitempty"`
	Hash     TOTPHash `json:"hash,omitempty"`
	Interval int      `json:"interval,omitempty"`
}
