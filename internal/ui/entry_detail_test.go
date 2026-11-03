// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/otp"
	"go.lucor.dev/paw/internal/paw"
)

func detailFixture(t *testing.T) *model.Item {
	t.Helper()
	item, err := model.NewLogin("GitHub", "dev@example.com", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	item.Scope = &cxf.CredentialScope{URLs: []string{"https://github.com"}, AndroidApps: []cxf.AndroidAppID{}}
	note, err := json.Marshal(cxf.Note{Type: cxf.CredentialTypeNote, Content: cxf.EditableField{FieldType: cxf.FieldTypeString, Value: "Personal account for development."}})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, note)
	created, modified := uint64(1705055040), uint64(1709475420)
	item.CreationAt, item.ModifiedAt = &created, &modified
	return item
}

func sshDetailFixture(t *testing.T) (*model.Item, cxf.SSHKey) {
	t.Helper()
	seed := make([]byte, ed25519.SeedSize)
	for index := range seed {
		seed[index] = byte(index)
	}
	private := ed25519.NewKeyFromSeed(seed)
	der, err := x509.MarshalPKCS8PrivateKey(private)
	require.NoError(t, err)
	credential := cxf.SSHKey{
		Type: cxf.CredentialTypeSSHKey, KeyType: "ssh-ed25519", KeyComment: "Deploy key",
		PrivateKey: base64.RawURLEncoding.EncodeToString(der),
	}
	item, err := model.NewSSHKey("Production server", credential, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	created, modified := uint64(1705055040), uint64(1709475420)
	item.CreationAt, item.ModifiedAt = &created, &modified
	return item, credential
}

func TestEntryDetailUsesFirstCredentialOfEachType(t *testing.T) {
	empty, err := model.DecodeCredentials(nil)
	require.NoError(t, err)
	require.Nil(t, empty.BasicAuth)
	require.Equal(t, -1, empty.BasicAuthIndex)
	require.Nil(t, empty.Note)
	require.Equal(t, -1, empty.NoteIndex)
	require.Nil(t, empty.TOTP)
	require.Nil(t, empty.SSHKey)
	require.Equal(t, -1, empty.SSHKeyIndex)
	_, sshCredential := sshDetailFixture(t)
	secondSSH := sshCredential
	secondSSH.KeyComment = "Second key"
	firstSSHRaw, err := json.Marshal(sshCredential)
	require.NoError(t, err)
	secondSSHRaw, err := json.Marshal(secondSSH)
	require.NoError(t, err)

	decoded, err := model.DecodeCredentials([]json.RawMessage{
		json.RawMessage(`{"type":"basic-auth","username":{"fieldType":"string","value":"first"}}`),
		json.RawMessage(`{"type":"basic-auth","username":{"fieldType":"string","value":"second"}}`),
		json.RawMessage(`{"type":"note","content":{"fieldType":"string","value":"first note"}}`),
		json.RawMessage(`{"type":"note","content":{"fieldType":"string","value":"second note"}}`),
		json.RawMessage(`{"type":"totp","secret":"JBSWY3DPEHPK3PXP","algorithm":"sha256","period":45,"digits":8}`),
		json.RawMessage(`{"type":"totp","secret":"JBSWY3DPEHPK3PXP","algorithm":"sha1","period":30,"digits":6}`),
		firstSSHRaw,
		secondSSHRaw,
	})
	require.NoError(t, err)
	require.NotNil(t, decoded.BasicAuth)
	require.Equal(t, "first", decoded.BasicAuth.Username.Value)
	require.Equal(t, 0, decoded.BasicAuthIndex)
	require.NotNil(t, decoded.Note)
	require.Equal(t, "first note", decoded.Note.Content.Value)
	require.Equal(t, 2, decoded.NoteIndex)
	require.NotNil(t, decoded.TOTP)
	require.Equal(t, "sha256", decoded.TOTP.Algorithm)
	require.NotNil(t, decoded.SSHKey)
	require.Equal(t, sshCredential.KeyComment, decoded.SSHKey.KeyComment)
	require.Equal(t, 6, decoded.SSHKeyIndex)
}

func TestEntryDetailSSHKeyDisclosureAndActions(t *testing.T) {
	require.Equal(t, "Production-server", sshKeyFilename("Production/server"))
	require.Equal(t, "ssh-key", sshKeyFilename(" .. "))
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item, credential := sshDetailFixture(t)
	edited := false
	view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() { edited = true }, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	window.Resize(fyne.NewSize(430, 926))

	texts := auditTexts(view)
	require.NotContains(t, texts, "SSH Key")
	require.Contains(t, texts, "ED25519")
	require.Equal(t, 1, strings.Count(strings.Join(texts, "\n"), item.Title))
	require.Contains(t, strings.Join(texts, "\n"), "SHA256:")
	require.NotContains(t, strings.Join(texts, "\n"), "BEGIN OPENSSH PRIVATE KEY")
	require.NotContains(t, strings.Join(texts, "\n"), credential.PrivateKey)

	buttons := map[string]*iconButton{}
	for _, object := range test.LaidOutObjects(view) {
		switch object := object.(type) {
		case *iconButton:
			buttons[object.AccessibilityLabel()] = object
		case *widget.Button:
			require.NotEqual(t, "Edit", object.Text)
		}
	}
	for _, label := range []string{"Show private key", "Copy fingerprint", "Copy public key", "Download public key", "Copy private key", "Download private key", "Entry actions"} {
		require.Contains(t, buttons, label)
	}

	_, publicKey, fingerprint, privateErr := model.FormatSSHKey(credential)
	require.NoError(t, privateErr)
	test.Tap(buttons["Copy fingerprint"])
	require.Equal(t, fingerprint, fyne.CurrentApp().Clipboard().Content())
	test.Tap(buttons["Copy public key"])
	require.Equal(t, publicKey, fyne.CurrentApp().Clipboard().Content())
	test.Tap(buttons["Copy private key"])
	require.Contains(t, fyne.CurrentApp().Clipboard().Content(), "BEGIN OPENSSH PRIVATE KEY")
	test.Tap(buttons["Show private key"])
	require.Contains(t, strings.Join(auditTexts(view), "\n"), "BEGIN OPENSSH PRIVATE KEY")
	test.Tap(buttons["Show private key"])
	require.NotContains(t, strings.Join(auditTexts(view), "\n"), "BEGIN OPENSSH PRIVATE KEY")
	test.Tap(buttons["Download public key"])
	saveDialog := window.Canvas().Overlays().Top()
	require.NotNil(t, saveDialog)
	var saveLists int
	for _, object := range test.LaidOutObjects(saveDialog) {
		if _, ok := object.(*widget.List); ok {
			saveLists++
		}
	}
	require.GreaterOrEqual(t, saveLists, 2)
	window.Canvas().Overlays().Remove(saveDialog)
	test.Tap(buttons["Entry actions"])
	require.Equal(t, []string{"Edit entry", "Delete entry"}, menuLabels(t, window))
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if control, ok := object.(fyne.Widget); ok {
			for _, rendered := range test.WidgetRenderer(control).Objects() {
				if text, ok := rendered.(*canvas.Text); ok && text.Text == "Edit entry" {
					test.Tap(object.(fyne.Tappable))
				}
			}
		}
	}
	require.True(t, edited)
}

func TestEntryDetailSSHKeyGolden(t *testing.T) {
	previousZone := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousZone })
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := test.NewTempWindow(t, nil)
			item, _ := sshDetailFixture(t)
			view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
			require.NoError(t, err)
			window.SetContent(view)
			window.Resize(fyne.NewSize(430, 926))
			test.AssertImageMatches(t, "entry-detail-ssh-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}

func TestEntryDetailCredentialsAndActions(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item := detailFixture(t)
	before, err := json.Marshal(item)
	require.NoError(t, err)
	back, edited, deleted := false, false, false
	view, err := NewEntryDetail(context.Background(), item, window, func() { back = true }, func() { edited = true }, func() { deleted = true })
	require.NoError(t, err)
	window.SetContent(view)
	window.Resize(fyne.NewSize(380, 760))
	buttons := map[string]*iconButton{}
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok {
			buttons[button.AccessibilityLabel()] = button
		}
	}
	require.NotContains(t, auditTexts(view), "synthetic-secret")
	require.NotContains(t, buttons, "Copy Website")
	require.Contains(t, buttons, "Copy Notes")
	require.Contains(t, buttons, "Open website")
	require.Contains(t, auditTexts(view), "dev@example.com")
	require.Contains(t, auditTexts(view), "Personal account for development.")
	test.Tap(buttons["Copy Password"])
	require.Equal(t, "synthetic-secret", fyne.CurrentApp().Clipboard().Content())
	test.Tap(buttons["Show Password"])
	require.Contains(t, auditTexts(view), "synthetic-secret")
	test.Tap(buttons["Show Password"])
	require.NotContains(t, auditTexts(view), "synthetic-secret")
	test.Tap(buttons["Copy Username"])
	require.Equal(t, "dev@example.com", fyne.CurrentApp().Clipboard().Content())
	test.Tap(buttons["Copy Notes"])
	require.Equal(t, "Personal account for development.", fyne.CurrentApp().Clipboard().Content())
	test.Tap(buttons["Entry actions"])
	require.Equal(t, []string{"Edit entry", "Delete entry"}, menuLabels(t, window))
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if control, ok := object.(fyne.Widget); ok {
			for _, rendered := range test.WidgetRenderer(control).Objects() {
				if text, ok := rendered.(*canvas.Text); ok && text.Text == "Edit entry" {
					test.Tap(object.(fyne.Tappable))
				}
			}
		}
	}
	require.True(t, edited)
	test.Tap(buttons["Entry actions"])
	for _, object := range test.LaidOutObjects(window.Canvas().Overlays().Top()) {
		if control, ok := object.(fyne.Widget); ok {
			for _, rendered := range test.WidgetRenderer(control).Objects() {
				if text, ok := rendered.(*canvas.Text); ok && text.Text == "Delete entry" {
					test.Tap(object.(fyne.Tappable))
				}
			}
		}
	}
	require.True(t, deleted)
	test.Tap(buttons["Back"])
	require.True(t, back)
	after, err := json.Marshal(item)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}

func TestEntryDetailPasswordAndNote(t *testing.T) {
	for _, kind := range []string{"password", "note"} {
		t.Run(kind, func(t *testing.T) {
			test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			var item *model.Item
			var err error
			if kind == "password" {
				item, err = model.NewPassword("Wi-Fi", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
			} else {
				item, err = model.NewNote("Notes", "A private note")
			}
			require.NoError(t, err)
			view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
			require.NoError(t, err)
			window.SetContent(view)
			texts := auditTexts(view)
			require.NotContains(t, texts, "Username")
			require.NotContains(t, texts, "Website")
			require.NotContains(t, texts, "Password type")
			buttons := map[string]*iconButton{}
			for _, object := range test.LaidOutObjects(view) {
				if button, ok := object.(*iconButton); ok {
					buttons[button.AccessibilityLabel()] = button
				}
			}
			require.Contains(t, buttons, "Entry actions")
			if kind == "note" {
				require.Contains(t, texts, "A private note")
				require.Contains(t, buttons, "Copy Notes")
				require.NotContains(t, buttons, "Copy Password")
			} else {
				require.NotContains(t, texts, "synthetic-secret")
				require.Contains(t, buttons, "Show Password")
				require.Contains(t, buttons, "Copy Password")
				test.Tap(buttons["Show Password"])
				require.Contains(t, auditTexts(view), "synthetic-secret")
				test.Tap(buttons["Show Password"])
				require.NotContains(t, auditTexts(view), "synthetic-secret")
				test.Tap(buttons["Copy Password"])
				require.Equal(t, "synthetic-secret", fyne.CurrentApp().Clipboard().Content())
			}
		})
	}
}

func TestEntryDetailOmitsEmptyOptionalFieldsAndPreservesLongNote(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	emptyLogin, err := model.NewLogin("Empty login", "", "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	loginView, err := NewEntryDetail(context.Background(), emptyLogin, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	loginTexts := auditTexts(loginView)
	for _, label := range []string{"Username", "Password", "Website", "Verification code", "Notes"} {
		require.NotContains(t, loginTexts, label)
	}

	note := detailItemFixture(t, "note")
	noteView, err := NewEntryDetail(context.Background(), note, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	var noteBody *widget.Label
	for _, object := range test.LaidOutObjects(noteView) {
		if label, ok := object.(*widget.Label); ok && strings.HasPrefix(label.Text, "\u00a0\u00a0line one\u00a0\u00a0") {
			noteBody = label
		}
	}
	require.NotNil(t, noteBody)
	require.Equal(t, fyne.TextWrapBreak, noteBody.Wrapping)
	require.Equal(t, "\u00a0\u00a0line one\u00a0\u00a0\nline two with a long unbroken value: abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", noteBody.Text)
	for _, object := range test.LaidOutObjects(noteView) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Copy Notes" {
			test.Tap(button)
			require.Equal(t, "  line one  \nline two with a long unbroken value: abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz", fyne.CurrentApp().Clipboard().Content())
			return
		}
	}
	t.Fatal("note copy action not found")
}

func TestEntryDetailTOTPCopyUsesCXFParameters(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item := detailFixture(t)
	credential := cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha256", Period: 45, Digits: 8}
	raw, err := json.Marshal(credential)
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, raw)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	view, err := NewEntryDetail(ctx, item, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	require.NotContains(t, auditTexts(view), credential.Secret)
	var copyCode *iconButton
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == "Copy verification code" {
			copyCode = button
		}
	}
	require.NotNil(t, copyCode)
	before := time.Now().UTC()
	test.Tap(copyCode)
	after := time.Now().UTC()
	first, err := otp.TOTPFromBase32(sha256.New, credential.Secret, before, 45, 8)
	require.NoError(t, err)
	second, err := otp.TOTPFromBase32(sha256.New, credential.Secret, after, 45, 8)
	require.NoError(t, err)
	require.Contains(t, []string{first, second}, fyne.CurrentApp().Clipboard().Content())
}

func TestEntryDetailTOTPCodeAndProgressShareRow(t *testing.T) {
	for _, width := range []float32{430, 320} {
		t.Run(fmt.Sprintf("width-%d", int(width)), func(t *testing.T) {
			test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			view, err := NewEntryDetail(ctx, loginTOTPFixture(t), window, func() {}, func() {}, func() {})
			require.NoError(t, err)
			window.SetContent(view)
			window.Resize(fyne.NewSize(width, 926))

			var code *widget.Label
			var progress *widget.ProgressBar
			var copyCode *iconButton
			for _, object := range test.LaidOutObjects(view) {
				switch object := object.(type) {
				case *widget.Label:
					if displayed := strings.ReplaceAll(object.Text, " ", ""); len(displayed) == 6 && strings.Trim(displayed, "0123456789") == "" {
						code = object
					}
				case *widget.ProgressBar:
					progress = object
				case *iconButton:
					if object.AccessibilityLabel() == "Copy verification code" {
						copyCode = object
					}
				}
			}
			require.NotNil(t, code)
			require.NotNil(t, progress)
			require.NotNil(t, copyCode)
			require.Regexp(t, `^[0-9]{3} [0-9]{3}$`, code.Text)
			require.Regexp(t, `^[0-9]+s$`, progress.TextFormatter())
			require.Greater(t, progress.Size().Width, float32(0))
			driver := fyne.CurrentApp().Driver()
			codePosition := driver.AbsolutePositionForObject(code)
			progressPosition := driver.AbsolutePositionForObject(progress)
			copyPosition := driver.AbsolutePositionForObject(copyCode)
			codeCenterY := codePosition.Y + code.Size().Height/2
			progressCenterY := progressPosition.Y + progress.Size().Height/2
			require.InDelta(t, codeCenterY, progressCenterY, 3)
			require.GreaterOrEqual(t, progressPosition.X, codePosition.X+code.Size().Width)
			require.GreaterOrEqual(t, copyPosition.X, progressPosition.X+progress.Size().Width)
			require.LessOrEqual(t, copyPosition.X+copyCode.Size().Width, width)
		})
	}
}

func loginTOTPFixture(t *testing.T) *model.Item {
	t.Helper()
	item := detailFixture(t)
	raw, err := json.Marshal(cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 30, Digits: 6})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, raw)
	return item
}

func TestEntryDetailHidesUnavailableTOTP(t *testing.T) {
	for _, credential := range []cxf.TOTP{
		{Type: cxf.CredentialTypeTOTP, Algorithm: "sha1", Period: 30, Digits: 6},
		{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "md5", Period: 30, Digits: 6},
		{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Algorithm: "sha1", Period: 0, Digits: 6},
	} {
		test.NewTempApp(t)
		window := test.NewTempWindow(t, nil)
		item := detailFixture(t)
		raw, err := json.Marshal(credential)
		require.NoError(t, err)
		item.Credentials = append(item.Credentials, raw)
		view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
		require.NoError(t, err)
		require.NotContains(t, auditTexts(view), "TOTP")
		require.NotContains(t, auditTexts(view), "Code unavailable")
	}
}

func TestEntryDetailHeaderAndNarrowLayout(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, kind := range []string{"login", "password", "note", "ssh"} {
			t.Run(variantName(variant)+"/"+kind, func(t *testing.T) {
				test.NewTempApp(t).Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
				window := test.NewTempWindow(t, nil)
				item := detailItemFixture(t, kind)
				view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
				require.NoError(t, err)
				window.SetContent(view)
				window.Resize(fyne.NewSize(320, 640))
				objects := test.LaidOutObjects(view)
				buttons := map[string]*iconButton{}
				var scroll *container.Scroll
				var title *widget.Label
				for _, object := range objects {
					switch object := object.(type) {
					case *iconButton:
						buttons[object.AccessibilityLabel()] = object
					case *container.Scroll:
						scroll = object
					case *Navigation:
						t.Fatal("detail view must hide bottom navigation")
					case *widget.Label:
						if object.Text == item.Title {
							title = object
						}
					}
				}
				require.NotNil(t, scroll)
				require.NotNil(t, title)
				require.Contains(t, buttons, "Back")
				require.Contains(t, buttons, "Entry actions")
				require.LessOrEqual(t, title.Position().Y, scroll.Position().Y)
				require.LessOrEqual(t, title.Position().X+title.Size().Width, window.Canvas().Size().Width)
			})
		}
	}
}

func TestEntryDetailSSHKeyPassphraseAndActions(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	_, credential := sshDetailFixture(t)
	item, err := model.NewSSHKey("Production server", credential, "key passphrase", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	buttons := map[string]*iconButton{}
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok {
			buttons[button.AccessibilityLabel()] = button
		}
	}
	require.NotContains(t, auditTexts(view), "key passphrase")
	require.Contains(t, buttons, "Show Passphrase")
	require.Contains(t, buttons, "Copy Passphrase")
	test.Tap(buttons["Show Passphrase"])
	require.Contains(t, auditTexts(view), "key passphrase")
	test.Tap(buttons["Copy Passphrase"])
	require.Equal(t, "key passphrase", fyne.CurrentApp().Clipboard().Content())
	test.Tap(buttons["Download private key"])
	require.NotNil(t, window.Canvas().Overlays().Top())
}

func TestEntryDetailSecretsClearWhenVaultLocks(t *testing.T) {
	for _, kind := range []string{"password", "ssh"} {
		t.Run(kind, func(t *testing.T) {
			test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			item := detailItemFixture(t, kind)
			if kind == "ssh" {
				require.NotEmpty(t, mustDecodeSSHKey(t, item).PrivateKey)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			reveal := "Show Password"
			secret := "synthetic-secret"
			if kind == "ssh" {
				reveal = "Show private key"
				secret = "BEGIN OPENSSH PRIVATE KEY"
			}
			var view fyne.CanvasObject
			var main *container.Scroll
			var current fyne.CanvasObject
			var visibleBeforeLock, visibleAfterLock string
			var createErr error
			fyne.DoAndWait(func() {
				var clear func()
				view, clear, createErr = newEntryDetailWithClear(ctx, item, window, func() {}, func() {}, func() {})
				if createErr != nil {
					return
				}
				window.SetContent(view)
				buttons := map[string]*iconButton{}
				for _, object := range test.LaidOutObjects(view) {
					if button, ok := object.(*iconButton); ok {
						buttons[button.AccessibilityLabel()] = button
					}
				}
				test.Tap(buttons[reveal])
				visibleBeforeLock = strings.Join(auditTexts(view), "\n")
				main = container.NewVScroll(widget.NewLabel("Vault locked"))
				application := &app{win: window, main: main, detailCancel: cancel, detailClear: clear}
				application.lockVault()
				current = window.Content()
				visibleAfterLock = strings.Join(auditTexts(view), "\n")
			})
			require.NoError(t, createErr)
			require.Contains(t, visibleBeforeLock, secret)
			require.Same(t, main, current)
			require.NotContains(t, visibleAfterLock, secret)
		})
	}
}

func detailItemFixture(t *testing.T, kind string) *model.Item {
	t.Helper()
	switch kind {
	case "login":
		return detailFixture(t)
	case "password":
		item, err := model.NewPassword("Wi-Fi", "synthetic-secret", model.PasswordSettings{Mode: paw.CustomPassword})
		require.NoError(t, err)
		setDetailFixtureDates(item)
		return item
	case "note":
		item, err := model.NewNote("Recovery note", "  line one  \nline two with a long unbroken value: abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz")
		require.NoError(t, err)
		setDetailFixtureDates(item)
		return item
	case "ssh":
		item, _ := sshDetailFixture(t)
		return item
	default:
		t.Fatalf("unknown item fixture: %s", kind)
		return nil
	}
}

func setDetailFixtureDates(item *model.Item) {
	created, modified := uint64(1705055040), uint64(1709475420)
	item.CreationAt, item.ModifiedAt = &created, &modified
}

func mustDecodeSSHKey(t *testing.T, item *model.Item) cxf.SSHKey {
	t.Helper()
	decoded, err := model.DecodeCredentials(item.Credentials)
	require.NoError(t, err)
	require.NotNil(t, decoded.SSHKey)
	return *decoded.SSHKey
}

func TestEntryDetailGolden(t *testing.T) {
	previousZone := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousZone })
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, kind := range []string{"login", "password", "note"} {
			t.Run(variantName(variant)+"/"+kind, func(t *testing.T) {
				app := test.NewTempApp(t)
				app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
				window := test.NewTempWindow(t, nil)
				view, err := NewEntryDetail(context.Background(), detailItemFixture(t, kind), window, func() {}, func() {}, func() {})
				require.NoError(t, err)
				window.SetContent(view)
				window.Resize(fyne.NewSize(430, 926))
				name := "entry-detail-" + variantName(variant) + ".png"
				if kind != "login" {
					name = "entry-detail-" + kind + "-" + variantName(variant) + ".png"
				}
				test.AssertImageMatches(t, name, window.Canvas().Capture())
			})
		}
	}
}

func TestEntryDetailLoginUsesLogicalSectionDividers(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	view, err := NewEntryDetail(ctx, loginTOTPFixture(t), window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	window.Resize(fyne.NewSize(430, 926))
	objects := test.LaidOutObjects(view)
	separators := 0
	for _, object := range objects {
		if _, ok := object.(*widget.Separator); ok {
			separators++
		}
	}
	require.Equal(t, 3, separators, "credentials, TOTP, Notes, and metadata are four separate groups")
}

func TestEntryDetailSSHKeyNarrowAndRevealBehavior(t *testing.T) {
	previousZone := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousZone })
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			window := test.NewTempWindow(t, nil)
			item, _ := sshDetailFixture(t)
			view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
			require.NoError(t, err)
			window.SetContent(view)
			window.Resize(fyne.NewSize(320, 640))
			objects := test.LaidOutObjects(view)
			var algorithm *widget.Label
			var sectionLabel *widget.Label
			for _, object := range objects {
				if label, ok := object.(*widget.Label); ok {
					switch label.Text {
					case "ED25519":
						algorithm = label
					case "SSH key":
						sectionLabel = label
					}
				}
			}
			require.NotNil(t, algorithm)
			require.NotNil(t, sectionLabel)
			require.Equal(t, fyne.TextWrapOff, algorithm.Wrapping)
			driver := fyne.CurrentApp().Driver()
			algorithmPosition := driver.AbsolutePositionForObject(algorithm)
			sectionPosition := driver.AbsolutePositionForObject(sectionLabel)
			require.Greater(t, algorithmPosition.X, sectionPosition.X+sectionLabel.Size().Width)
			require.InDelta(t, algorithmPosition.Y+algorithm.Size().Height/2, sectionPosition.Y+sectionLabel.Size().Height/2, 3)
			require.LessOrEqual(t, algorithmPosition.X+algorithm.Size().Width, window.Canvas().Size().Width)
			test.AssertImageMatches(t, "entry-detail-ssh-narrow-"+variantName(variant)+".png", window.Canvas().Capture())

			buttons := detailIconButtons(t, view)
			require.Contains(t, buttons, "Show private key")
			require.Contains(t, buttons, "Copy private key")
			require.Contains(t, buttons, "Download private key")
			test.Tap(buttons["Show private key"])
			objects = test.LaidOutObjects(view)
			var privateSurface *widget.Card
			var privateText *widget.Label
			for _, object := range objects {
				if card, ok := object.(*widget.Card); ok && card.Visible() {
					privateSurface = card
				}
				if label, ok := object.(*widget.Label); ok && strings.HasPrefix(label.Text, "-----BEGIN OPENSSH PRIVATE KEY-----") {
					privateText = label
				}
			}
			require.NotNil(t, privateSurface)
			require.NotNil(t, privateText)
			require.True(t, privateText.Visible())
			require.Greater(t, privateText.Size().Width, float32(0))
			require.Contains(t, strings.Join(auditTexts(view), "\n"), "BEGIN OPENSSH PRIVATE KEY")
			window.Canvas().Refresh(view)
			test.Tap(buttons["Show private key"])
			require.NotContains(t, strings.Join(auditTexts(view), "\n"), "BEGIN OPENSSH PRIVATE KEY")
		})
	}
}

func detailIconButtons(t *testing.T, view fyne.CanvasObject) map[string]*iconButton {
	t.Helper()
	buttons := make(map[string]*iconButton)
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok {
			buttons[button.AccessibilityLabel()] = button
		}
	}
	return buttons
}

func TestEntryDetailMetadataStaysAtBottomForShortItems(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, kind := range []string{"login", "password", "note", "ssh"} {
			t.Run(variantName(variant)+"/"+kind, func(t *testing.T) {
				app := test.NewTempApp(t)
				app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
				window := test.NewTempWindow(t, nil)
				ctx, cancel := context.WithCancel(context.Background())
				t.Cleanup(cancel)
				view, err := NewEntryDetail(ctx, detailItemFixture(t, kind), window, func() {}, func() {}, func() {})
				require.NoError(t, err)
				window.SetContent(view)
				for _, height := range []float32{926, 640} {
					window.Resize(fyne.NewSize(430, height))
					scroll, _ := detailScrollAndModifiedLabel(t, view)
					require.LessOrEqual(t, scroll.Content.MinSize().Height, scroll.Size().Height,
						"fixture should remain short at the tested viewport height")
					requireMetadataAtScrollBottom(t, scroll)
				}
			})
		}
	}
}

func TestEntryDetailMetadataRowsHaveCompactSpacing(t *testing.T) {
	previousZone := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousZone })
	for _, kind := range []string{"login", "password", "note", "ssh"} {
		t.Run(kind, func(t *testing.T) {
			test.NewTempApp(t)
			window := test.NewTempWindow(t, nil)
			item := detailItemFixture(t, kind)
			createdBefore, modifiedBefore := *item.CreationAt, *item.ModifiedAt
			view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
			require.NoError(t, err)
			window.SetContent(view)
			window.Resize(fyne.NewSize(430, 640))
			objects := test.LaidOutObjects(view)
			var metadata *widget.Label
			for _, object := range objects {
				label, ok := object.(*widget.Label)
				if !ok {
					continue
				}
				if strings.HasPrefix(label.Text, "Created · ") {
					metadata = label
				}
			}
			require.NotNil(t, metadata)
			require.Equal(t, "Created · 12 Jan 2024, 10:24:00\nModified · 3 Mar 2024, 14:17:00", metadata.Text,
				"all item types should show compact local timestamps with seconds")
			require.Equal(t, createdBefore, *item.CreationAt)
			require.Equal(t, modifiedBefore, *item.ModifiedAt)
		})
	}
}

func TestFormatDetailTimestampUsesLocalTimeAndSeconds(t *testing.T) {
	previousZone := time.Local
	time.Local = time.FixedZone("test local", 2*60*60)
	t.Cleanup(func() { time.Local = previousZone })

	timestamp := uint64(time.Date(2026, time.November, 2, 15, 45, 21, 987654321, time.UTC).Unix())
	require.Equal(t, "2 Nov 2026, 17:45:21", formatDetailTimestamp(timestamp))
}

func TestEntryDetailOmitsMissingTimestamps(t *testing.T) {
	test.NewTempApp(t)
	item := detailItemFixture(t, "password")
	item.CreationAt = nil
	item.ModifiedAt = nil
	window := test.NewTempWindow(t, nil)
	view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	require.NotContains(t, auditTexts(view), "Created · ")
	require.NotContains(t, auditTexts(view), "Modified · ")
}

func TestEntryDetailLongNoteMetadataFollowsScrollableContent(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	content := strings.Repeat("A long secure note line with details that must remain readable.\n", 80)
	item, err := model.NewNote("Long note", content)
	require.NoError(t, err)
	setDetailFixtureDates(item)
	view, err := NewEntryDetail(context.Background(), item, window, func() {}, func() {}, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	window.Resize(fyne.NewSize(430, 640))
	scroll, modified := detailScrollAndModifiedLabel(t, view)
	require.Greater(t, scroll.Content.MinSize().Height, scroll.Size().Height)
	viewportBottom := fyne.CurrentApp().Driver().AbsolutePositionForObject(scroll).Y + scroll.Size().Height
	modifiedPosition := fyne.CurrentApp().Driver().AbsolutePositionForObject(modified)
	require.Greater(t, modifiedPosition.Y, viewportBottom,
		"metadata should follow the complete note outside the initial viewport")
	for _, object := range test.LaidOutObjects(view) {
		if nested, ok := object.(*container.Scroll); ok {
			require.Same(t, scroll, nested, "detail content must have one scroll container")
		}
	}
	scroll.ScrollToBottom()
	requireMetadataAtScrollBottom(t, scroll)
}

func detailScrollAndModifiedLabel(t *testing.T, view fyne.CanvasObject) (*container.Scroll, *widget.Label) {
	t.Helper()
	objects := test.LaidOutObjects(view)
	var scroll *container.Scroll
	var modified *widget.Label
	scrollCount := 0
	for _, object := range objects {
		switch object := object.(type) {
		case *container.Scroll:
			scroll = object
			scrollCount++
		case *widget.Label:
			if strings.Contains(object.Text, "\nModified · ") {
				modified = object
			}
		}
	}
	require.Equal(t, 1, scrollCount, "detail view must use one scroll container")
	require.NotNil(t, scroll)
	require.NotNil(t, modified)
	return scroll, modified
}

func requireMetadataAtScrollBottom(t *testing.T, scroll *container.Scroll) {
	t.Helper()
	scrollPosition := fyne.CurrentApp().Driver().AbsolutePositionForObject(scroll)
	scrollBottom := scrollPosition.Y + scroll.Size().Height
	body, ok := scroll.Content.(*fyne.Container)
	require.True(t, ok)
	require.NotEmpty(t, body.Objects)
	require.GreaterOrEqual(t, len(body.Objects), 2)
	require.IsType(t, &widget.Separator{}, body.Objects[len(body.Objects)-2],
		"metadata should have one divider immediately above it")
	metadata := body.Objects[len(body.Objects)-1]
	metadataPosition := fyne.CurrentApp().Driver().AbsolutePositionForObject(metadata)
	metadataBottom := metadataPosition.Y + metadata.Size().Height
	require.InDelta(t, scrollBottom, metadataBottom, 1,
		"short item metadata should align with the bottom of the available scroll content")
}
