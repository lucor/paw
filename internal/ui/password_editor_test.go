// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	json "encoding/json/v2"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func passwordEditorFixture(t *testing.T) *model.Item {
	t.Helper()
	item, err := model.NewPassword("Wi-Fi", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	totp, err := json.Marshal(cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 30, Digits: 6}, json.Deterministic(true))
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, totp)
	return item
}

func TestPasswordEditorControlsAndInteractions(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	editor, err := NewPasswordEditor(passwordEditorFixture(t), false)
	require.NoError(t, err)
	window.SetContent(editor.View)
	window.Resize(fyne.NewSize(430, 926))
	require.Equal(t, "Edit password", editor.headerTitle.Text)
	require.Nil(t, editor.Title.Validator)
	require.True(t, editor.Password.Entry.Password)
	require.True(t, editor.Password.strength.Visible())
	require.Less(t, editor.GenerateButton.Size().Height, editor.Password.Size().Height)
	labels := make([]string, len(editor.form.Items))
	for index, item := range editor.form.Items {
		labels[index] = item.Text
	}
	require.Equal(t, []string{"Title", "Password"}, labels)
	for _, object := range test.LaidOutObjects(editor.View) {
		_, isNavigation := object.(*Navigation)
		_, isSelect := object.(*widget.Select)
		require.False(t, isNavigation)
		require.False(t, isSelect)
	}
}

func TestPasswordEditorUpdatesPasswordAndPreservesOtherCredentials(t *testing.T) {
	test.NewTempApp(t)
	item := passwordEditorFixture(t)
	before, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	editor, err := NewPasswordEditor(item, false)
	require.NoError(t, err)
	editor.Title.SetText("Office Wi-Fi")
	editor.Password.Entry.SetText("another-synthetic-secret")
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	require.Equal(t, item.ID, updated.ID)
	require.Equal(t, "Office Wi-Fi", updated.Title)
	decoded, err := model.DecodeCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "another-synthetic-secret", decoded.BasicAuth.Password.Value)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.TOTP.Secret)
	after, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}

func TestPasswordEditorKeepsManuallyModifiedGeneratedPassword(t *testing.T) {
	test.NewTempApp(t)
	editor, err := NewPasswordEditor(passwordEditorFixture(t), false)
	require.NoError(t, err)

	editor.Password.Entry.SetText("GeneratedPassword-123!")
	editor.PasswordSpec = &paw.PasswordSpec{Mode: paw.RandomPassword, Value: "GeneratedPassword-123!", Length: 22, Format: paw.LowercaseFormat | paw.UppercaseFormat | paw.DigitsFormat | paw.SymbolsFormat}
	unchanged, err := editor.UpdatedItem()
	require.NoError(t, err)
	unchangedCredentials, err := model.DecodeCredentials(unchanged.Credentials)
	require.NoError(t, err)
	require.Equal(t, paw.RandomPassword, model.PasswordSettingsFromField(unchangedCredentials.BasicAuth.Password).Mode)
	generatedScore := editor.Password.strength.score
	editor.Password.Entry.SetText("manually changed")

	require.NotEqual(t, generatedScore, editor.Password.strength.score)
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := model.DecodeCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "manually changed", decoded.BasicAuth.Password.Value)
	require.Equal(t, paw.CustomPassword, model.PasswordSettingsFromField(decoded.BasicAuth.Password).Mode)
	reopened, err := NewPasswordEditor(updated, false)
	require.NoError(t, err)
	require.Equal(t, paw.CustomPassword, reopened.PasswordSpec.Mode)
	require.Equal(t, "manually changed", reopened.Password.Entry.Text)
}

func TestPasswordEditorValidatesTitleAndTracksDirtyState(t *testing.T) {
	test.NewTempApp(t)
	editor, err := NewPasswordEditor(passwordEditorFixture(t), true)
	require.NoError(t, err)
	require.Equal(t, "New password", editor.headerTitle.Text)
	require.Empty(t, editor.Title.Text)
	require.True(t, editor.SaveButton.Disabled())
	require.False(t, editor.IsDirty())
	editor.Title.SetText(" ")
	_, err = editor.UpdatedItem()
	require.EqualError(t, err, "title is required")
	require.True(t, editor.HasValidationErrors())
	editor.Title.SetText("Personal")
	require.False(t, editor.SaveButton.Disabled())
	require.False(t, editor.HasValidationErrors())
	require.True(t, editor.IsDirty())
	_, err = editor.UpdatedItem()
	require.NoError(t, err)
}

func TestPasswordEditorGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := test.NewTempWindow(t, nil)
			editor, err := NewPasswordEditor(passwordEditorFixture(t), false)
			require.NoError(t, err)
			window.SetContent(editor.View)
			window.Resize(fyne.NewSize(430, 926))
			test.AssertImageMatches(t, "password-editor-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}
