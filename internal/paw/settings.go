// SPDX-FileCopyrightText: 2024-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

const (
	ThemeSystem                      = "system"
	ThemeLight                       = "light"
	ThemeDark                        = "dark"
	StartupVaultLastUsed             = "last_used"
	StartupVaultChooseAtStartup      = "choose_at_startup"
	DefaultTheme                     = ThemeSystem
	DefaultStartupVault              = StartupVaultLastUsed
	AutoLockNever                    = "never"
	AutoLockOneMinute                = "1m"
	AutoLockFiveMinutes              = "5m"
	AutoLockTenMinutes               = "10m"
	AutoLockFifteenMinutes           = "15m"
	AutoLockThirtyMinutes            = "30m"
	AutoLockOneHour                  = "1h"
	AutoLockTwoHours                 = "2h"
	AutoLockFourHours                = "4h"
	AutoLockEightHours               = "8h"
	AutoLockOneDay                   = "24h"
	FaviconDownloaderDisabledDefault = false
)

// NewDefaultSettings returns the default application settings.
func NewDefaultSettings() *Settings {
	return &Settings{
		Theme:        DefaultTheme,
		StartupVault: DefaultStartupVault,
		AutoLock:     AutoLockNever,
		FaviconDownloader: FaviconDownloaderSettings{
			Disabled: FaviconDownloaderDisabledDefault,
		},
		Password: PasswordSettings{
			Passphrase: PassphrasePasswordSettings{
				DefaultLength: PassphrasePasswordDefaultLength,
				MaxLength:     PassphrasePasswordMaxLength,
				MinLength:     PassphrasePasswordMinLength,
			},
			Pin: PinPasswordSettings{
				DefaultLength: PinPasswordDefaultLength,
				MaxLength:     PinPasswordMaxLength,
				MinLength:     PinPasswordMinLength,
			},
			Random: RandomPasswordSettings{
				DefaultLength: RandomPasswordDefaultLength,
				DefaultFormat: RandomPasswordDefaultFormat,
				MaxLength:     RandomPasswordMaxLength,
				MinLength:     RandomPasswordMinLength,
			},
		},
	}
}

type Settings struct {
	Theme             string                    `json:"theme,omitempty"`
	StartupVault      string                    `json:"startup_vault,omitempty"`
	AutoLock          string                    `json:"auto_lock,omitempty"`
	LastVaultID       string                    `json:"last_vault_id,omitempty"`
	FaviconDownloader FaviconDownloaderSettings `json:"favicon_downloader,omitempty"`
	Password          PasswordSettings          `json:"password,omitempty"`
}

// FaviconDownloaderSettings configures automatic favicon downloads.
// FaviconDownloader tool is opt-out, hence the default value is false.
type FaviconDownloaderSettings struct {
	Disabled bool `json:"disabled,omitempty"` // Disabled is true if the favicon downloader is disabled.
}

type PasswordSettings struct {
	Passphrase PassphrasePasswordSettings `json:"passphrase,omitempty"`
	Pin        PinPasswordSettings        `json:"pin,omitempty"`
	Random     RandomPasswordSettings     `json:"random,omitempty"`
}

type PassphrasePasswordSettings struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}

type PinPasswordSettings struct {
	DefaultLength int `json:"default_length,omitempty"`
	MaxLength     int `json:"max_length,omitempty"`
	MinLength     int `json:"min_length,omitempty"`
}
type RandomPasswordSettings struct {
	DefaultLength int    `json:"default_length,omitempty"`
	DefaultFormat Format `json:"default_format,omitempty"`
	MaxLength     int    `json:"max_length,omitempty"`
	MinLength     int    `json:"min_length,omitempty"`
}
