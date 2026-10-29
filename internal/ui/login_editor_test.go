// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func loginEditorFixture(t *testing.T) *model.Item {
	t.Helper()
	item := detailFixture(t)
	totp, err := json.Marshal(cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 30, Digits: 6})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, totp)
	return item
}

func newTestLoginEditor(t *testing.T, item *model.Item, isNew bool) *LoginEditor {
	t.Helper()
	preferences := paw.NewDefaultPreferences()
	preferences.FaviconDownloader.Disabled = true
	editor, err := NewLoginEditor(context.Background(), item, preferences.FaviconDownloader, preferences.TOTP, isNew)
	require.NoError(t, err)
	return editor
}

func TestLoginEditorControlsAndInteractions(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	window.SetContent(editor.View)
	window.Resize(fyne.NewSize(430, 926))
	require.True(t, editor.Password.strength.Visible())
	require.Equal(t, "Edit login", editor.headerTitle.Text)
	require.Nil(t, editor.Title.Validator)
	require.Nil(t, editor.Website.Validator)

	buttons := map[string]*iconButton{}
	for _, object := range test.LaidOutObjects(editor.View) {
		if button, ok := object.(*iconButton); ok {
			buttons[button.AccessibilityLabel()] = button
		}
	}
	require.Contains(t, buttons, "Back")
	require.Contains(t, buttons, "Generate password")
	require.True(t, editor.Password.Entry.Password, "password should use Fyne's native masked entry")
	require.Less(t, editor.GenerateButton.Size().Height, editor.Password.Size().Height,
		"generator action should align with the entry instead of the strength row")
	generated := false
	editor.GenerateButton.OnTapped = func() { generated = true }
	test.Tap(editor.GenerateButton)
	require.True(t, generated)

	require.Equal(t, "Save", editor.SaveButton.Text)
	formLabels := make([]string, len(editor.form.Items))
	for index, item := range editor.form.Items {
		formLabels[index] = item.Text
	}
	require.Equal(t, []string{"Title", "Username", "Password", "Website", "TOTP secret", "Notes"}, formLabels)
	for _, object := range test.LaidOutObjects(editor.View) {
		_, isNavigation := object.(*Navigation)
		require.False(t, isNavigation, "entry editor should hide bottom navigation")
	}
}

func TestFaviconPreviewUsesGlobeForMissingFavicon(t *testing.T) {
	test.NewTempApp(t)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	editor.Website.FaviconListener(nil)
	require.Equal(t, icons.Globe.Name(), editor.favicon.icon.Resource.Name())
	editor.Website.FaviconListener(&paw.Favicon{Host: "example.com"})
	require.Equal(t, icons.Globe.Name(), editor.favicon.icon.Resource.Name())
}

func TestNewLoginEditorHeaderAndDirtyState(t *testing.T) {
	test.NewTempApp(t)
	item, err := model.NewLogin("Untitled login", "", "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	editor := newTestLoginEditor(t, item, true)
	require.Equal(t, "New login", editor.headerTitle.Text)
	require.False(t, editor.IsDirty())
	editor.Title.SetText("Personal email")
	require.True(t, editor.IsDirty())
}

func TestLoginEditorUpdatesCredentialsAndPreservesTOTP(t *testing.T) {
	test.NewTempApp(t)
	item := loginEditorFixture(t)
	before, err := json.Marshal(item)
	require.NoError(t, err)
	editor := newTestLoginEditor(t, item, false)
	editor.Title.SetText("GitHub work")
	editor.Username.SetText("work@example.com")
	editor.Password.Entry.SetText("new-synthetic-secret")
	editor.Website.SetText("https://example.org")
	editor.Notes.SetText("Updated private note")

	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	require.Equal(t, item.ID, updated.ID)
	require.Equal(t, "GitHub work", updated.Title)
	require.Equal(t, "work@example.com", *updated.Subtitle)
	require.Equal(t, []string{"https://example.org"}, updated.Scope.URLs)
	decoded, err := decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "work@example.com", decoded.basicAuth.Username.Value)
	require.Equal(t, "new-synthetic-secret", decoded.basicAuth.Password.Value)
	require.Equal(t, "Updated private note", decoded.note.Content.Value)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.totp.Secret)
	var oldTOTP cxf.TOTP
	var newTOTP cxf.TOTP
	require.NoError(t, json.Unmarshal(item.Credentials[2], &oldTOTP))
	require.NoError(t, json.Unmarshal(updated.Credentials[2], &newTOTP))
	require.Equal(t, oldTOTP, newTOTP)
	after, err := json.Marshal(item)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}

func TestLoginEditorAddsTOTPUsingPreferences(t *testing.T) {
	test.NewTempApp(t)
	item, err := model.NewLogin("GitHub", "dev@example.com", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	preferences := paw.NewDefaultPreferences()
	preferences.FaviconDownloader.Disabled = true
	preferences.TOTP.Hash = paw.SHA256
	preferences.TOTP.Digits = 8
	preferences.TOTP.Interval = 45
	editor, err := NewLoginEditor(context.Background(), item, preferences.FaviconDownloader, preferences.TOTP, true)
	require.NoError(t, err)
	editor.TOTPSecret.SetText("jbsw y3dp-ehpk3pxp")
	editor.Title.SetText("GitHub")
	require.True(t, editor.IsDirty())
	require.True(t, editor.TOTPSecret.Password, "TOTP secret should be masked by default")
	test.Tap(editor.TOTPReveal)
	require.False(t, editor.TOTPSecret.Password)
	test.Tap(editor.TOTPReveal)
	require.True(t, editor.TOTPSecret.Password)

	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.totp.Secret)
	require.Equal(t, cxf.CredentialTypeTOTP, decoded.totp.Type)
	require.Equal(t, "sha256", decoded.totp.Algorithm)
	require.Equal(t, uint16(45), decoded.totp.Period)
	require.Equal(t, uint16(8), decoded.totp.Digits)
}

func TestLoginEditorUpdatesAndRemovesTOTP(t *testing.T) {
	test.NewTempApp(t)
	item := loginEditorFixture(t)
	editor := newTestLoginEditor(t, item, false)
	editor.TOTPSecret.SetText("not base32!")
	if _, err := editor.UpdatedItem(); err == nil {
		t.Fatal("expected invalid TOTP secret to be rejected")
	}
	require.True(t, editor.totpError.Visible())
	require.Equal(t, "enter a valid Base32 secret", editor.totpError.Text)

	editor.TOTPSecret.SetText("MZXW6YTBOI======")
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "MZXW6YTBOI======", decoded.totp.Secret)
	require.Equal(t, "sha1", decoded.totp.Algorithm)
	require.Equal(t, uint16(30), decoded.totp.Period)
	require.Equal(t, uint16(6), decoded.totp.Digits)

	editor.TOTPSecret.SetText("")
	updated, err = editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err = decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Nil(t, decoded.totp, "clearing the optional secret removes the TOTP credential")
}

func TestLoginEditorValidatesTitleAndWebsite(t *testing.T) {
	test.NewTempApp(t)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	editor.Title.SetText(" ")
	_, err := editor.UpdatedItem()
	require.EqualError(t, err, "title is required")
	require.True(t, editor.titleError.Visible())
	editor.Title.SetText("GitHub")
	require.False(t, editor.titleError.Visible())
	editor.Website.SetText("http://example.org")
	_, err = editor.UpdatedItem()
	require.EqualError(t, err, "website must use HTTPS")
	require.True(t, editor.websiteError.Visible())
	editor.Website.SetText("https://example.org")
	require.False(t, editor.websiteError.Visible())
	_, err = editor.UpdatedItem()
	require.NoError(t, err)
}

func TestLoginEditorGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := test.NewTempWindow(t, nil)
			editor := newTestLoginEditor(t, loginEditorFixture(t), false)
			window.SetContent(editor.View)
			window.Resize(fyne.NewSize(430, 926))
			test.AssertImageMatches(t, "login-editor-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}

func TestLoginEditorWebsiteIsOptional(t *testing.T) {
	test.NewTempApp(t)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	editor.Website.SetText("")
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	require.Empty(t, updated.Scope.URLs[0])
	require.True(t, strings.TrimSpace(updated.Title) != "")
}

func TestLoginEditorSaveButtonIsNative(t *testing.T) {
	test.NewTempApp(t)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	require.NotNil(t, editor.SaveButton)
	require.Equal(t, "Save", editor.SaveButton.Text)
	require.Equal(t, widget.HighImportance, editor.SaveButton.Importance)
}
