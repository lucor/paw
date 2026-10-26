// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package migration

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestMigrateLoginPreservesLoginData checks the fields mapped from a legacy login.
func TestMigrateLoginPreservesLoginData(t *testing.T) {
	login := paw.NewLogin()
	login.Name = "GitHub"
	login.Metadata.Subtitle = "Saved subtitle"
	login.Username = "jane@example.com"
	login.Password.Value = "secret"
	login.Password.Mode = paw.RandomPassword
	login.Password.Length = 24
	login.Password.Format = paw.LowercaseFormat | paw.DigitsFormat
	login.Password.Note.Value = "ignored password note"
	login.Password.Metadata.Favicon = &paw.Favicon{Host: "ignored.example"}
	login.TOTP = &paw.TOTP{Secret: "JBSWY3DPEHPK3PXP", Digits: 6, Hash: paw.SHA256, Interval: 30}
	login.Note.Value = "login note"
	login.URL = paw.NewLoginURL()
	require.NoError(t, login.URL.Set("http://example.com/login"))
	login.Metadata.Created = time.Unix(10, 123).UTC()
	login.Metadata.Modified = time.Unix(20, 456).UTC()
	login.Metadata.Favicon = &paw.Favicon{Host: "example.com", Data: []byte("icon"), Format: "png"}

	item, err := MigrateLogin(login)
	require.NoError(t, err)
	require.NotEmpty(t, item.ID)
	require.NotEqual(t, login.ID(), item.ID)
	require.Equal(t, paw.LoginItemType, item.Metadata.Type)
	require.Equal(t, "GitHub", item.Title)
	require.Equal(t, "Saved subtitle", *item.Subtitle)
	require.Equal(t, uint64(10), *item.CreationAt)
	require.Equal(t, uint64(20), *item.ModifiedAt)
	require.Equal(t, []string{"http://example.com/login"}, item.Scope.URLs)
	require.NotNil(t, item.Scope.AndroidApps)
	require.Equal(t, &paw.Favicon{Host: "example.com", Data: []byte("icon"), Format: "png"}, item.Metadata.Favicon)
	login.Metadata.Favicon.Data[0] = 'X'
	require.Equal(t, []byte("icon"), item.Metadata.Favicon.Data)
	require.Len(t, item.Credentials, 3)

	var basic cxf.BasicAuth
	require.NoError(t, json.Unmarshal(item.Credentials[0], &basic))
	require.Equal(t, "jane@example.com", basic.Username.Value)
	require.Equal(t, "secret", basic.Password.Value)
	var passwordSettings model.PasswordSettings
	var passwordExtension struct {
		Name string `json:"name"`
		model.PasswordSettings
	}
	require.NoError(t, json.Unmarshal(basic.Password.Extensions[0], &passwordExtension))
	passwordSettings = passwordExtension.PasswordSettings
	require.Equal(t, model.PasswordSettings{Mode: paw.RandomPassword, Length: 24, Format: paw.LowercaseFormat | paw.DigitsFormat}, passwordSettings)

	var totp cxf.TOTP
	require.NoError(t, json.Unmarshal(item.Credentials[1], &totp))
	require.Equal(t, "sha256", totp.Algorithm)
	var note cxf.Note
	require.NoError(t, json.Unmarshal(item.Credentials[2], &note))
	require.Equal(t, "login note", note.Content.Value)
	require.NotContains(t, string(item.Credentials[2]), "ignored password note")
	require.NotContains(t, string(item.Credentials[0]), "ignored.example")
	require.NoError(t, item.Validate())

	encoded, err := json.Marshal(item)
	require.NoError(t, err)
	var decoded model.Item
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, item.Metadata, decoded.Metadata)
	require.Equal(t, item.Scope, decoded.Scope)
	summary, err := decoded.Summary()
	require.NoError(t, err)
	require.Equal(t, item.Scope, summary.Scope)
	require.Empty(t, summary.Credentials)
	summaryJSON := mustJSON(t, summary)
	require.Contains(t, string(summaryJSON), "example.com")
	require.NotContains(t, string(summaryJSON), "secret")
	var summaryItem model.Item
	require.NoError(t, json.Unmarshal(summaryJSON, &summaryItem))
	require.Equal(t, item.Metadata.Favicon, summaryItem.Metadata.Favicon)
}

// TestMigrateLoginHandlesOptionalValues checks absent and empty legacy fields.
func TestMigrateLoginHandlesOptionalValues(t *testing.T) {
	login := paw.NewLogin()
	login.Name = "Empty"
	login.Username = ""
	login.Password = nil
	login.TOTP = paw.NewDefaultTOTP()
	login.Note = &paw.Note{}
	login.URL = nil
	login.Metadata.Favicon = nil
	login.Metadata.Created = time.Time{}
	login.Metadata.Modified = time.Time{}

	item, err := MigrateLogin(login)
	require.NoError(t, err)
	require.Nil(t, item.CreationAt)
	require.Nil(t, item.ModifiedAt)
	require.Nil(t, item.Scope)
	require.Nil(t, item.Metadata.Favicon)
	require.Len(t, item.Credentials, 1)
	var basic cxf.BasicAuth
	require.NoError(t, json.Unmarshal(item.Credentials[0], &basic))
	require.Empty(t, basic.Username.Value)
	require.Empty(t, basic.Password.Value)
	require.NoError(t, item.Validate())
}

// TestMigrateLoginRejectsInvalidData checks errors omit secret values.
func TestMigrateLoginRejectsInvalidData(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*paw.Login)
	}{
		{name: "invalid metadata", setup: func(login *paw.Login) { login.Metadata = nil }},
		{name: "invalid password mode", setup: func(login *paw.Login) {
			login.Password.Mode = paw.PasswordMode(4)
			login.Password.Value = "do-not-report"
		}},
		{name: "invalid TOTP", setup: func(login *paw.Login) { login.TOTP.Secret = "invalid-secret" }},
		{name: "invalid note UTF-8", setup: func(login *paw.Login) { login.Note.Value = "bad\xff" }},
	} {
		t.Run(test.name, func(t *testing.T) {
			login := paw.NewLogin()
			login.Name = "Example"
			test.setup(login)
			item, err := MigrateLogin(login)
			require.Error(t, err)
			require.Nil(t, item)
			require.NotContains(t, err.Error(), "do-not-report")
			require.NotContains(t, err.Error(), "invalid-secret")
		})
	}
}

// mustJSON encodes a test value and fails the test if encoding fails.
func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return data
}
