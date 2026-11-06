// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	json "encoding/json/v2"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestSSHKeyEditorControlsAndPrivateKeyDisclosure(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item, _ := sshDetailFixture(t)
	editor, err := NewSSHKeyEditor(item, false)
	require.NoError(t, err)
	window.SetContent(editor.View)
	window.Resize(fyne.NewSize(430, 926))
	require.Equal(t, "Edit SSH key", editor.headerTitle.Text)
	require.True(t, editor.Passphrase.Password)
	require.Empty(t, editor.privateBox.Objects, "private key must be hidden initially")
	require.NotNil(t, icons.SSH)

	buttons := map[string]fyne.Tappable{}
	for _, object := range test.LaidOutObjects(editor.View) {
		if button, ok := object.(interface{ AccessibilityLabel() string }); ok {
			if tappable, ok := object.(fyne.Tappable); ok {
				buttons[button.AccessibilityLabel()] = tappable
			}
		}
	}
	for _, label := range []string{"Back", "Show passphrase", "Copy passphrase", "Copy fingerprint", "Copy public key", "Export public key", "Reveal private key", "Copy private key", "Export private key"} {
		require.Contains(t, buttons, label)
	}
	labels := make([]string, len(editor.form.Items))
	for index, item := range editor.form.Items {
		labels[index] = item.Text
	}
	require.Equal(t, []string{"Title", "Key passphrase"}, labels)
	for _, object := range test.LaidOutObjects(editor.View) {
		_, isNavigation := object.(*Navigation)
		_, isSelect := object.(*widget.Select)
		require.False(t, isNavigation)
		require.False(t, isSelect)
	}
	test.Tap(editor.RevealPassphraseButton)
	require.False(t, editor.Passphrase.Password, "the passphrase should be visible after Show")
	test.Tap(editor.RevealPassphraseButton)
	require.True(t, editor.Passphrase.Password, "the passphrase should be masked again after Hide")

	scrollCountBefore := 0
	for _, object := range test.LaidOutObjects(editor.View) {
		if _, ok := object.(*container.Scroll); ok {
			scrollCountBefore++
		}
	}
	test.Tap(editor.RevealPrivateButton)
	require.NotNil(t, editor.privateLabel)
	require.True(t, strings.HasPrefix(editor.privateLabel.Text, "-----BEGIN OPENSSH PRIVATE KEY-----\n"))
	require.True(t, strings.HasSuffix(editor.privateLabel.Text, "-----END OPENSSH PRIVATE KEY-----\n"))
	require.True(t, editor.privateLabel.TextStyle.Monospace)
	require.True(t, editor.privateLabel.Selectable)
	require.Equal(t, fyne.TextWrapBreak, editor.privateLabel.Wrapping)
	scrollCount := 0
	for _, object := range test.LaidOutObjects(editor.View) {
		if _, ok := object.(*container.Scroll); ok {
			scrollCount++
		}
	}
	require.Equal(t, scrollCountBefore, scrollCount, "revealing the key should not add a nested scroll container")
	test.Tap(editor.RevealPrivateButton)
	require.Nil(t, editor.privateLabel)
	require.Empty(t, editor.privateBox.Objects)
}

func TestSSHKeyEditorLayoutAcrossWindowWidths(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := test.NewTempWindow(t, nil)
			item, _ := sshDetailFixture(t)
			editor, err := NewSSHKeyEditor(item, false)
			require.NoError(t, err)
			window.SetContent(editor.View)
			for _, size := range []fyne.Size{fyne.NewSize(440, 926), fyne.NewSize(320, 640)} {
				window.Resize(size)
				objects := test.LaidOutObjects(editor.View)
				var keyHeading *widget.Label
				var replacementHint *widget.Label
				var keyHeadingRow *fyne.Container
				separators := 0
				for _, object := range objects {
					switch object := object.(type) {
					case *fyne.Container:
						for _, child := range object.Objects {
							if child == editor.keyType {
								keyHeadingRow = object
							}
						}
					case *widget.Label:
						if object.Text == "ED25519" {
							keyHeading = object
						}
						if strings.HasPrefix(object.Text, "Generate or import replaces the key") {
							replacementHint = object
						}
					case *widget.Separator:
						separators++
					}
				}
				require.NotNil(t, keyHeading)
				require.NotNil(t, replacementHint)
				require.NotNil(t, keyHeadingRow)
				require.Equal(t, fyne.TextWrapOff, keyHeading.Wrapping)
				require.Equal(t, fyne.TextWrapOff, replacementHint.Wrapping)
				require.Equal(t, "Generate or import replaces the key on save.", replacementHint.Text)
				require.Equal(t, fyne.TextWrapBreak, editor.publicPreview.Wrapping)
				require.Equal(t, 4, separators, "dividers should separate the key sections")

				driver := fyne.CurrentApp().Driver()
				keyHeadingRowPosition := driver.AbsolutePositionForObject(keyHeadingRow)
				keyHeadingPosition := driver.AbsolutePositionForObject(keyHeading)
				require.InDelta(t, keyHeadingRowPosition.X+keyHeadingRow.Size().Width,
					keyHeadingPosition.X+keyHeading.Size().Width, 1, "the algorithm should align to the right edge")
				generatePosition := driver.AbsolutePositionForObject(editor.GenerateButton)
				importPosition := driver.AbsolutePositionForObject(editor.ImportButton)
				require.InDelta(t, editor.GenerateButton.Size().Width, editor.ImportButton.Size().Width, 1)
				require.InDelta(t, editor.GenerateButton.Size().Height, editor.ImportButton.Size().Height, 1)
				require.InDelta(t, generatePosition.Y, importPosition.Y, 1)
				require.GreaterOrEqual(t, importPosition.X, generatePosition.X+editor.GenerateButton.Size().Width)
				require.LessOrEqual(t, importPosition.X+editor.ImportButton.Size().Width, size.Width)
				require.Equal(t, widget.MediumImportance, editor.GenerateButton.Importance)
				require.Equal(t, widget.MediumImportance, editor.ImportButton.Importance)
				require.Equal(t, icons.GenerateKey, editor.GenerateButton.Icon)
				require.Equal(t, icons.ImportKey, editor.ImportButton.Icon)
				hintPosition := driver.AbsolutePositionForObject(replacementHint)
				require.LessOrEqual(t, hintPosition.X+replacementHint.Size().Width, size.Width,
					"helper text should stay within the view width")
				require.LessOrEqual(t, fyne.MeasureText(replacementHint.Text, theme.Size(theme.SizeNameCaptionText), fyne.TextStyle{}).Width,
					replacementHint.Size().Width, "helper text should fit on one line")
			}
		})
	}
}

func TestSSHKeyEditorUpdatesCredentialsAndClearsSensitiveValues(t *testing.T) {
	test.NewTempApp(t)
	item, originalKey := sshDetailFixture(t)
	original, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	editor, err := NewSSHKeyEditor(item, false)
	require.NoError(t, err)
	editor.Title.SetText("Updated production key")
	editor.Passphrase.SetText("new passphrase")
	editor.AddToAgent.SetChecked(true)
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	require.Equal(t, item.ID, updated.ID)
	require.Equal(t, "Updated production key", updated.Title)
	require.True(t, updated.Metadata.AddToAgent)
	decoded, err := model.DecodeCredentials(updated.Credentials)
	require.NoError(t, err)
	require.Equal(t, originalKey.PrivateKey, decoded.SSHKey.PrivateKey)
	require.Equal(t, "new passphrase", decoded.BasicAuth.Password.Value)

	unchanged, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(original), string(unchanged), "the editor must not mutate its input item")
	require.True(t, editor.IsDirty())
	require.False(t, editor.HasValidationErrors())

	editor.ClearSensitive()
	require.Empty(t, editor.Passphrase.Text)
	require.Nil(t, editor.sshKey)
	require.Empty(t, editor.initialPrivate)
	require.Empty(t, editor.initialPass)
	require.Empty(t, editor.publicText)
	require.Empty(t, editor.publicPreview.Text)
	require.Nil(t, editor.item)
	require.Nil(t, editor.CopyPrivateButton.OnTapped)
	afterClear, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	require.JSONEq(t, string(original), string(afterClear), "clearing the editor must not mutate its input item")
}

func TestSSHKeyEditorValidatesTitleAndTracksNewHeader(t *testing.T) {
	test.NewTempApp(t)
	item, err := model.NewSSHKey("New SSH key", sshDetailFixtureKey(t), "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	editor, err := NewSSHKeyEditor(item, true)
	require.NoError(t, err)
	require.Equal(t, "New SSH key", editor.headerTitle.Text)
	require.Empty(t, editor.Title.Text)
	require.True(t, editor.SaveButton.Disabled())
	require.False(t, editor.IsDirty())
	editor.Title.SetText("  ")
	_, err = editor.UpdatedItem()
	require.EqualError(t, err, "title is required")
	require.True(t, editor.HasValidationErrors())
	editor.Title.SetText("Admin key")
	require.False(t, editor.SaveButton.Disabled())
	require.False(t, editor.HasValidationErrors())
	require.True(t, strings.HasPrefix(editor.PublicKey(), "ssh-ed25519 "))
}

func sshDetailFixtureKey(t *testing.T) cxf.SSHKey {
	t.Helper()
	_, key := sshDetailFixture(t)
	return key
}

func TestSSHKeyEditorGenerationUpdatesDisplayedAndSavedKey(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item, _ := sshDetailFixture(t)
	editor, err := NewSSHKeyEditor(item, false)
	require.NoError(t, err)
	editor.ConnectWindow(window)
	window.SetContent(editor.View)
	editor.Passphrase.SetText("keep this passphrase")
	editor.PasswordSpec.Mode = paw.PassphrasePassword
	editor.PasswordSpec.Length = 5
	before := editor.Fingerprint()
	test.Tap(editor.GenerateButton)
	require.NotEqual(t, before, editor.Fingerprint())
	require.Equal(t, editor.Fingerprint(), editor.fingerprintLabel.Text)
	test.Tap(editor.CopyFingerprintButton)
	require.Equal(t, editor.Fingerprint(), fyne.CurrentApp().Clipboard().Content())
	updated, err := editor.UpdatedItem()
	require.NoError(t, err)
	decoded, err := model.DecodeCredentials(updated.Credentials)
	require.NoError(t, err)
	_, public, fingerprint, err := model.FormatSSHKey(*decoded.SSHKey)
	require.NoError(t, err)
	require.Equal(t, editor.PublicKey(), public)
	require.Equal(t, editor.Fingerprint(), fingerprint)
	require.Equal(t, "keep this passphrase", decoded.BasicAuth.Password.Value)
	settings := passwordSpecFromField(decoded.BasicAuth.Password)
	require.Equal(t, paw.PassphrasePassword, settings.Mode)
	require.Equal(t, 5, settings.Length)
}

func TestSSHKeyImportPickerUsesSharedDesktopDefaults(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item, _ := sshDetailFixture(t)
	editor, err := NewSSHKeyEditor(item, false)
	require.NoError(t, err)
	editor.ConnectWindow(window)
	window.Resize(fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight))
	window.SetContent(editor.View)
	test.Tap(editor.ImportButton)

	picker := window.Canvas().Overlays().Top()
	require.NotNil(t, picker)
	require.Equal(t, fyne.NewSize(DefaultWindowWidth, DefaultWindowHeight), picker.Size())
	var lists int
	for _, object := range test.LaidOutObjects(picker) {
		if _, ok := object.(*widget.List); ok {
			lists++
		}
	}
	require.GreaterOrEqual(t, lists, 2)
	window.Canvas().Overlays().Remove(picker)
}
