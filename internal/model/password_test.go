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

// TestNewPasswordPreservesValueAndSettings checks CXF password encoding.
func TestNewPasswordPreservesValueAndSettings(t *testing.T) {
	for _, mode := range []paw.PasswordMode{
		paw.CustomPassword,
		paw.RandomPassword,
		paw.PassphrasePassword,
		paw.PinPassword,
	} {
		t.Run(mode.String(), func(t *testing.T) {
			settings := PasswordSettings{Mode: mode, Length: 24, Format: paw.LowercaseFormat | paw.DigitsFormat}
			item, err := NewPassword("Example", "", settings)
			require.NoError(t, err)
			require.Equal(t, paw.PasswordItemType, item.Metadata.Type)
			require.Len(t, item.Credentials, 1)

			var credential cxf.BasicAuth
			require.NoError(t, json.Unmarshal(item.Credentials[0], &credential))
			require.Equal(t, cxf.CredentialTypeBasicAuth, credential.Type)
			require.Nil(t, credential.Username)
			require.NotNil(t, credential.Password)
			require.Equal(t, cxf.FieldTypeConcealedString, credential.Password.FieldType)
			require.Empty(t, credential.Password.Value)
			require.Len(t, credential.Password.Extensions, 1)

			var extension struct {
				Name string `json:"name"`
				PasswordSettings
			}
			require.NoError(t, json.Unmarshal(credential.Password.Extensions[0], &extension))
			require.Equal(t, passwordExtensionName, extension.Name)
			require.Equal(t, settings, extension.PasswordSettings)

			encoded, err := json.Marshal(item)
			require.NoError(t, err)
			var decoded Item
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, item.ID, decoded.ID)
			require.Equal(t, item.Metadata.Type, decoded.Metadata.Type)
			require.Equal(t, item.Credentials, decoded.Credentials)
		})
	}
}

// TestNewPasswordSummaryOmitsCredentials checks summaries contain no secret data.
func TestNewPasswordSummaryOmitsCredentials(t *testing.T) {
	item, err := NewPassword("Example", "secret-value", PasswordSettings{
		Mode: paw.RandomPassword, Length: 16, Format: paw.RandomPasswordDefaultFormat,
	})
	require.NoError(t, err)

	summary, err := item.Summary()
	require.NoError(t, err)
	require.Empty(t, summary.Credentials)
	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "secret-value")
	require.NotContains(t, string(encoded), passwordExtensionName)
}

// TestNewPasswordRejectsInvalidInput checks invalid text and modes fail safely.
func TestNewPasswordRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		name     string
		title    string
		value    string
		settings PasswordSettings
	}{
		{name: "invalid title", title: "bad\xff"},
		{name: "invalid value", title: "Title", value: "bad\xff"},
		{name: "unsupported mode", title: "Title", settings: PasswordSettings{Mode: paw.PasswordMode(4)}},
	} {
		t.Run(test.name, func(t *testing.T) {
			item, err := NewPassword(test.title, test.value, test.settings)
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "bad")
		})
	}
}
