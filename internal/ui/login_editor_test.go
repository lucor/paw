// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"encoding/json"
	"strconv"
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
	settings := model.NewDefaultSettings()
	settings.FaviconDownloader.Disabled = true
	editor, err := NewLoginEditor(context.Background(), item, settings.FaviconDownloader, isNew)
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
	require.Contains(t, buttons, "Authenticator settings")
	require.NotContains(t, buttons, "Show TOTP secret", "the password entry supplies its native reveal control")
	var settingsIcon string
	fyne.DoAndWait(func() { settingsIcon = buttons["Authenticator settings"].Icon.Name() })
	require.Equal(t, icons.Settings2.Name(), settingsIcon)
	require.True(t, editor.Password.Entry.Password, "password should use Fyne's native masked entry")
	require.NotNil(t, editor.TOTPSecret.Validator, "TOTP secret uses native focus-loss validation")
	require.NotNil(t, editor.TOTPSecret.ActionItem, "TOTP secret uses the native password reveal control")
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
	require.Equal(t, []string{"Title", "Username", "Password", "Website", "Authenticator key", "Notes"}, formLabels)
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
	editor.Website.FaviconListener(&model.Favicon{Host: "example.com"})
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

func TestLoginEditorAddsTOTPUsingStandardDefaults(t *testing.T) {
	test.NewTempApp(t)
	item, err := model.NewLogin("GitHub", "dev@example.com", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	settings := model.NewDefaultSettings()
	settings.FaviconDownloader.Disabled = true
	editor, err := NewLoginEditor(context.Background(), item, settings.FaviconDownloader, true)
	require.NoError(t, err)
	const enteredSecret = "jbsw \ty3dp-\nehpk3pxp"
	editor.TOTPSecret.SetText(enteredSecret)
	require.Equal(t, enteredSecret, editor.TOTPSecret.Text, "editing preserves the user's input")
	editor.Title.SetText("GitHub")
	require.True(t, editor.IsDirty())
	require.True(t, editor.TOTPSecret.Password, "TOTP secret should be masked by default")
	reveal, ok := editor.TOTPSecret.ActionItem.(fyne.Tappable)
	require.True(t, ok, "password entry reveal control should be tappable")
	test.Tap(reveal)
	require.False(t, editor.TOTPSecret.Password)
	test.Tap(reveal)
	require.True(t, editor.TOTPSecret.Password)

	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "JBSWY3DPEHPK3PXP", decoded.totp.Secret)
	require.Equal(t, cxf.CredentialTypeTOTP, decoded.totp.Type)
	require.Equal(t, strings.ToLower(string(paw.TOTPHashDefault)), decoded.totp.Algorithm)
	require.Equal(t, uint16(paw.TOTPIntervalDefault), decoded.totp.Period)
	require.Equal(t, uint16(paw.TOTPDigitsDefault), decoded.totp.Digits)
}

func TestLegacyTOTPPreferencesDoNotAffectNewAuthenticatorDefaults(t *testing.T) {
	test.NewTempApp(t)
	var state model.AppState
	require.NoError(t, json.Unmarshal([]byte(`{"settings":{"totp":{"hash":"SHA256","digits":8,"interval":45}}}`), &state))
	require.NotNil(t, state.Settings)
	item, err := model.NewLogin("GitHub", "dev@example.com", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	editor, err := NewLoginEditor(context.Background(), item, state.Settings.FaviconDownloader, true)
	require.NoError(t, err)
	editor.Title.SetText("GitHub")
	editor.TOTPSecret.SetText("JBSWY3DPEHPK3PXP")
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, strings.ToLower(string(paw.TOTPHashDefault)), decoded.totp.Algorithm)
	require.Equal(t, uint16(paw.TOTPDigitsDefault), decoded.totp.Digits)
	require.Equal(t, uint16(paw.TOTPIntervalDefault), decoded.totp.Period)
}

func TestTOTPSecretNormalizationPreservesInvalidCharacters(t *testing.T) {
	for _, testCase := range []struct {
		name  string
		input string
		want  string
	}{
		{name: "canonical", input: "JBSWY3DPEHPK3PXP", want: "JBSWY3DPEHPK3PXP"},
		{name: "whitespace-and-case", input: "jbsw \ty3dp\r\nehpk3pxp", want: "JBSWY3DPEHPK3PXP"},
		{name: "mixed-case", input: "JbswY3dpehpk3pxp", want: "JBSWY3DPEHPK3PXP"},
		{name: "hyphen-separated", input: "jbsw-y3dpehpk3pxp", want: "JBSWY3DPEHPK3PXP"},
		{name: "ambiguous-digit-preserved", input: "JBSW0Y3DPEHPK3PXP", want: "JBSW0Y3DPEHPK3PXP"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			require.Equal(t, testCase.want, normalizeTOTPSecret(testCase.input))
		})
	}

	test.NewTempApp(t)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	require.Error(t, editor.validateTOTPSecret("JBSW!Y3DPEHPK3PXP"), "invalid characters must remain invalid")
	require.NoError(t, editor.validateTOTPSecret("JBSW-Y3DPEHPK3PXP"), "hyphens are removed as formatting separators")
}

func TestTOTPSettingsDialogUsesStandardDefaultsAndAppliesOnlyOnApply(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item, err := model.NewLogin("GitHub", "dev@example.com", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	editor, err := NewLoginEditor(context.Background(), item, model.NewDefaultSettings().FaviconDownloader, true)
	require.NoError(t, err)
	require.Equal(t, strings.ToLower(string(paw.TOTPHashDefault)), editor.totpSettings.Algorithm)
	require.Equal(t, uint16(paw.TOTPDigitsDefault), editor.totpSettings.Digits)
	require.Equal(t, uint16(paw.TOTPIntervalDefault), editor.totpSettings.Period)

	showTOTPSettingsDialog(editor, window)
	popup := totpDialogOverlay(window)
	require.NotNil(t, popup)
	require.Contains(t, totpDialogTexts(popup), totpSettingsDialogTitle)
	algorithm, digits := totpDialogSelects(t, popup)
	algorithmValue, digitsValue := totpDialogSelections(algorithm, digits)
	require.Equal(t, string(paw.TOTPHashDefault), algorithmValue)
	require.Equal(t, strconv.Itoa(paw.TOTPDigitsDefault), digitsValue)
	require.Equal(t, paw.TOTPIntervalDefault, totpDialogInterval(t, popup))
	require.Contains(t, totpDialogTexts(popup), "Interval")
	var intervalLabelWidth, intervalLabelMinimum float32
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(popup) {
			if label, ok := object.(*widget.Label); ok && label.Text == "Interval" {
				intervalLabelWidth = label.Size().Width
				intervalLabelMinimum = label.MinSize().Width
			}
		}
	})
	require.GreaterOrEqual(t, intervalLabelWidth, intervalLabelMinimum)
	setTOTPDialogSelections(algorithm, digits, "SHA512", "9")
	test.Tap(totpDialogButton(t, popup, icons.Plus))
	require.Equal(t, uint16(paw.TOTPIntervalDefault), editor.totpSettings.Period, "dialog edits remain isolated until Apply")
	test.Tap(totpDialogAction(t, popup, "Apply"))
	require.Nil(t, totpDialogOverlay(window))
	require.Equal(t, "sha512", editor.totpSettings.Algorithm)
	require.Equal(t, uint16(9), editor.totpSettings.Digits)
	require.Equal(t, uint16(paw.TOTPIntervalDefault+totpIntervalStep), editor.totpSettings.Period)
	require.True(t, editor.IsDirty())

	showTOTPSettingsDialog(editor, window)
	popup = totpDialogOverlay(window)
	algorithm, digits = totpDialogSelects(t, popup)
	algorithmValue, digitsValue = totpDialogSelections(algorithm, digits)
	require.Equal(t, "SHA512", algorithmValue)
	require.Equal(t, "9", digitsValue)
	require.Equal(t, paw.TOTPIntervalDefault+totpIntervalStep, totpDialogInterval(t, popup))
	setTOTPDialogSelections(algorithm, digits, "SHA1", digitsValue)
	test.Tap(totpDialogAction(t, popup, "Cancel"))
	require.Nil(t, totpDialogOverlay(window))
	require.Equal(t, "sha512", editor.totpSettings.Algorithm, "Cancel discards changes since opening")
}

func TestTOTPSettingsDialogLoadsStoredParametersAndPreservesNonFiveInterval(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item := loginEditorFixture(t)
	var stored cxf.TOTP
	require.NoError(t, json.Unmarshal(item.Credentials[2], &stored))
	stored.Algorithm, stored.Digits, stored.Period = "sha512", 9, 47
	item.Credentials[2], _ = json.Marshal(stored)
	editor, err := NewLoginEditor(context.Background(), item, model.NewDefaultSettings().FaviconDownloader, false)
	require.NoError(t, err)
	require.Equal(t, stored, editor.totpSettings, "editing uses the stored parameters, not current defaults")
	updatedItem, err := editor.UpdatedItem()
	require.NoError(t, err)
	updatedCredentials, err := decodeEntryDetailCredentials(updatedItem.Credentials)
	require.NoError(t, err)
	require.Equal(t, stored, *updatedCredentials.totp, "saving without changes preserves imported per-item parameters")

	showTOTPSettingsDialog(editor, window)
	popup := totpDialogOverlay(window)
	algorithm, digits := totpDialogSelects(t, popup)
	algorithmValue, digitsValue := totpDialogSelections(algorithm, digits)
	require.Equal(t, "SHA512", algorithmValue)
	require.Equal(t, "9", digitsValue)
	require.Equal(t, 47, totpDialogInterval(t, popup))
	test.Tap(totpDialogButton(t, popup, icons.Plus))
	require.Equal(t, 52, totpDialogInterval(t, popup))
	test.Tap(totpDialogAction(t, popup, "Cancel"))
	require.Equal(t, stored, editor.totpSettings)

	showTOTPSettingsDialog(editor, window)
	popup = totpDialogOverlay(window)
	algorithm, digits = totpDialogSelects(t, popup)
	setTOTPDialogSelections(algorithm, digits, "SHA1", "7")
	test.Tap(totpDialogAction(t, popup, "Apply"))
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := decodeEntryDetailCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, "sha1", decoded.totp.Algorithm)
	require.Equal(t, uint16(7), decoded.totp.Digits)
	require.Equal(t, uint16(47), decoded.totp.Period, "Apply preserves valid intervals that are not a multiple of five")
}

func TestTOTPSettingsDialogRejectsUnsupportedParameters(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	editor := newTestLoginEditor(t, loginEditorFixture(t), false)
	editor.totpSettings.Algorithm = "unsupported"
	showTOTPSettingsDialog(editor, window)
	popup := totpDialogOverlay(window)
	test.Tap(totpDialogAction(t, popup, "Apply"))
	require.NotNil(t, totpDialogOverlay(window), "invalid settings keep the dialog open")
	require.Contains(t, totpDialogTexts(popup), "select a supported TOTP algorithm")
	require.Equal(t, "unsupported", editor.totpSettings.Algorithm, "Cancel preserves the editor's current value")
	test.Tap(totpDialogAction(t, popup, "Cancel"))
}

func totpDialogSelects(t *testing.T, dialogObject fyne.CanvasObject) (*widget.Select, *widget.Select) {
	t.Helper()
	var algorithm, digits *widget.Select
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(dialogObject) {
			selectWidget, ok := object.(*widget.Select)
			if !ok {
				continue
			}
			if len(selectWidget.Options) == len(totpHashOptions) {
				algorithm = selectWidget
			} else if len(selectWidget.Options) == len(totpDigitOptions) {
				digits = selectWidget
			}
		}
	})
	require.NotNil(t, algorithm)
	require.NotNil(t, digits)
	return algorithm, digits
}

func totpDialogInterval(t *testing.T, dialogObject fyne.CanvasObject) int {
	t.Helper()
	var interval int
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(dialogObject) {
			if label, ok := object.(*widget.Label); ok && label.TextStyle.Monospace {
				interval, _ = strconv.Atoi(label.Text)
			}
		}
	})
	if interval != 0 {
		return interval
	}
	t.Fatal("TOTP interval stepper is missing")
	return 0
}

func totpDialogButton(t *testing.T, dialogObject fyne.CanvasObject, resource fyne.Resource) *widget.Button {
	t.Helper()
	var found *widget.Button
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(dialogObject) {
			if button, ok := object.(*widget.Button); ok && button.Icon != nil && button.Icon.Name() == resource.Name() {
				found = button
			}
		}
	})
	if found != nil {
		return found
	}
	t.Fatalf("TOTP dialog button %q is missing", resource.Name())
	return nil
}

func totpDialogAction(t *testing.T, dialogObject fyne.CanvasObject, title string) *widget.Button {
	t.Helper()
	var found *widget.Button
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(dialogObject) {
			if button, ok := object.(*widget.Button); ok && button.Text == title {
				found = button
			}
		}
	})
	if found != nil {
		return found
	}
	t.Fatalf("TOTP dialog action %q is missing", title)
	return nil
}

func totpDialogOverlay(window fyne.Window) fyne.CanvasObject {
	var overlay fyne.CanvasObject
	fyne.DoAndWait(func() { overlay = window.Canvas().Overlays().Top() })
	return overlay
}

func totpDialogTexts(view fyne.CanvasObject) []string {
	var texts []string
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(view) {
			if label, ok := object.(*widget.Label); ok && label.Visible() {
				texts = append(texts, label.Text)
			}
		}
	})
	return texts
}

func totpDialogSelections(algorithm, digits *widget.Select) (string, string) {
	var algorithmValue, digitsValue string
	fyne.DoAndWait(func() {
		algorithmValue = algorithm.Selected
		digitsValue = digits.Selected
	})
	return algorithmValue, digitsValue
}

func setTOTPDialogSelections(algorithm, digits *widget.Select, algorithmValue, digitsValue string) {
	fyne.DoAndWait(func() {
		algorithm.SetSelected(algorithmValue)
		digits.SetSelected(digitsValue)
	})
}

func TestLoginEditorUpdatesAndRemovesTOTP(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item := loginEditorFixture(t)
	editor := newTestLoginEditor(t, item, false)
	window.SetContent(editor.View)
	window.Canvas().Focus(editor.TOTPSecret)
	editor.TOTPSecret.SetText("not base32!")
	require.False(t, editor.totpError.Visible(), "invalid text is not reported while the user is typing")
	window.Canvas().Focus(editor.Title)
	require.True(t, editor.totpError.Visible(), "invalid text is reported when the field loses focus")
	var errorSize fyne.Size
	fyne.DoAndWait(func() {
		errorSize = editor.totpError.Size()
	})
	require.Greater(t, errorSize.Height, float32(0), "validation error should occupy visible form layout space")
	if _, err := editor.UpdatedItem(); err == nil {
		t.Fatal("expected invalid TOTP secret to be rejected")
	}
	require.True(t, editor.totpError.Visible())
	require.Equal(t, "enter a valid secret", editor.totpError.Text)

	editor.TOTPSecret.SetText("MZXW6YTBOI======")
	require.False(t, editor.totpError.Visible(), "a corrected secret clears the error immediately")
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
