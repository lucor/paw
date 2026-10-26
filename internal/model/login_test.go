// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
)

// TestNewLoginBuildsBasicAuth checks the login credential and password settings.
func TestNewLoginBuildsBasicAuth(t *testing.T) {
	for _, mode := range []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			settings := PasswordSettings{Mode: mode, Length: 24, Format: paw.LowercaseFormat | paw.DigitsFormat}
			item, err := NewLogin("GitHub", "jane", "secret", settings)
			require.NoError(t, err)
			require.Equal(t, paw.LoginItemType, item.Metadata.Type)
			require.Len(t, item.Credentials, 1)

			var credential cxf.BasicAuth
			require.NoError(t, json.Unmarshal(item.Credentials[0], &credential))
			require.Equal(t, "jane", credential.Username.Value)
			require.Equal(t, cxf.FieldTypeString, credential.Username.FieldType)
			require.Equal(t, "secret", credential.Password.Value)
			require.Equal(t, cxf.FieldTypeConcealedString, credential.Password.FieldType)
			var settingsExtension passwordExtension
			require.NoError(t, json.Unmarshal(credential.Password.Extensions[0], &settingsExtension))
			require.Equal(t, settings, settingsExtension.PasswordSettings)
		})
	}
}

// TestNewLoginRejectsInvalidInput checks malformed text and unsupported modes fail.
func TestNewLoginRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name     string
		title    string
		username string
		password string
		settings PasswordSettings
	}{
		{name: "title", title: "bad\xff"},
		{name: "username", title: "Title", username: "bad\xff"},
		{name: "password", title: "Title", password: "bad\xff"},
		{name: "mode", title: "Title", settings: PasswordSettings{Mode: paw.PasswordMode(4)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			item, err := NewLogin(test.title, test.username, test.password, test.settings)
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "bad")
			if test.password != "" {
				require.NotContains(t, err.Error(), test.password)
			}
		})
	}
}
