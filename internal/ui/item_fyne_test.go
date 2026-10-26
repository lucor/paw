// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"
	"golang.org/x/crypto/ssh"

	"go.lucor.dev/paw/internal/icon"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

// TestFyneItemWidgetEditsCXFNote checks the Note editor saves CXF content.
func TestFyneItemWidgetEditsCXFNote(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	item, err := model.NewNote("Old title", "old text")
	require.NoError(t, err)
	widget := NewFyneItemWidget(item, &paw.Preferences{}).(*modelItemWidget)
	widget.Edit(context.Background(), nil, app.NewWindow("Edit note"))
	widget.title.SetText("New title")
	widget.note.SetText("new text")
	updated, err := widget.OnSubmit()
	require.NoError(t, err)
	require.Equal(t, "New title", updated.Title)
	var note cxf.Note
	require.NoError(t, json.Unmarshal(updated.Credentials[0], &note))
	require.Equal(t, "new text", note.Content.Value)
}

// TestFyneItemWidgetShowsLoginWithoutTOTP checks a login without TOTP can be rendered.
func TestFyneItemWidgetShowsLoginWithoutTOTP(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	item, err := model.NewLogin("Website", "user", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	itemWidget := NewFyneItemWidget(item, paw.NewDefaultPreferences())
	view := newItemViewWidget(context.Background(), itemWidget, app.NewWindow("Login"))

	require.NotPanics(t, func() { view.CreateRenderer() })
}

// TestFyneItemWidgetEditsLoginHTTPSAndTOTP checks HTTPS and TOTP editing.
func TestFyneItemWidgetEditsLoginHTTPSAndTOTP(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	item, err := model.NewLogin("Old title", "old-user", "old-password", model.PasswordSettings{Mode: paw.RandomPassword, Length: 20, Format: paw.RandomPasswordDefaultFormat})
	require.NoError(t, err)
	item.Scope = &cxf.CredentialScope{URLs: []string{"https://old.example.com"}, AndroidApps: []cxf.AndroidAppID{}}
	totp, err := json.Marshal(cxf.TOTP{Type: cxf.CredentialTypeTOTP, Secret: "JBSWY3DPEHPK3PXP", Period: 30, Digits: 6, Algorithm: "sha1"})
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, totp)
	prefs := paw.NewDefaultPreferences()
	prefs.FaviconDownloader.Disabled = true
	widget := NewFyneItemWidget(item, prefs).(*modelItemWidget)
	widget.Edit(context.Background(), nil, app.NewWindow("Edit login"))
	widget.title.SetText("Work login")
	widget.website.SetText("http://insecure.example.com")
	_, err = widget.OnSubmit()
	require.ErrorContains(t, err, "must use HTTPS")
	widget.website.SetText("https://secure.example.com")
	widget.username.SetText("new-user")
	widget.password.SetText("new-password")
	widget.totpSpec.Secret = "KRSXG5DSNFXGOIDB"
	widget.totpSpec.Hash = paw.SHA256
	widget.totpSpec.Digits = 8
	widget.totpSpec.Interval = 45
	updated, err := widget.OnSubmit()
	require.NoError(t, err)
	require.Equal(t, "Work login", updated.Title)
	require.Equal(t, []string{"https://secure.example.com"}, updated.Scope.URLs)
	login, _, err := widgetBasicAuth(updated)
	require.NoError(t, err)
	require.Equal(t, "new-user", login.Username.Value)
	require.Equal(t, "new-password", login.Password.Value)
	updatedTOTP, _, err := widgetTOTP(updated)
	require.NoError(t, err)
	require.Equal(t, "KRSXG5DSNFXGOIDB", updatedTOTP.Secret)
	require.Equal(t, paw.SHA256, updatedTOTP.Hash)
	require.Equal(t, 8, updatedTOTP.Digits)
	require.Equal(t, 45, updatedTOTP.Interval)
}

// TestFyneItemWidgetDownloadsLoginFavicon saves a favicon found on the login page.
func TestFyneItemWidgetDownloadsLoginFavicon(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var iconData bytes.Buffer
	iconImage := image.NewRGBA(image.Rect(0, 0, 16, 16))
	iconImage.Set(8, 8, color.RGBA{G: 0xff, A: 0xff})
	require.NoError(t, png.Encode(&iconData, iconImage))
	var requestCount int32
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		switch r.URL.Path {
		case "/account/login":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><head><link rel="icon" href="assets/favicon.png"></head></html>`))
		case "/account/assets/favicon.png":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(iconData.Bytes())
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	defaultClient := http.DefaultClient
	http.DefaultClient = server.Client()
	t.Cleanup(func() { http.DefaultClient = defaultClient })

	item, err := model.NewLogin("Website", "user", "password", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	itemWidget := NewFyneItemWidget(item, paw.NewDefaultPreferences()).(*modelItemWidget)
	window := app.NewWindow("Edit login")
	window.SetContent(itemWidget.Edit(context.Background(), nil, window))
	itemWidget.website.SetText(server.URL + "/account/login")
	faviconDownloaded := make(chan struct{}, 1)
	listener := itemWidget.websiteEntry.FaviconListener
	itemWidget.websiteEntry.FaviconListener = func(favicon *paw.Favicon) {
		listener(favicon)
		if favicon != nil {
			faviconDownloaded <- struct{}{}
		}
	}
	window.Canvas().Focus(itemWidget.websiteEntry)
	window.Canvas().Focus(itemWidget.username)
	select {
	case <-faviconDownloaded:
	case <-time.After(time.Second):
		t.Fatal("favicon download did not complete")
	}
	require.NotNil(t, itemWidget.loginFavicon.Resource)

	updated, err := itemWidget.OnSubmit()
	require.NoError(t, err)
	require.NotNil(t, updated.Metadata.Favicon)
	require.Equal(t, "127.0.0.1", updated.Metadata.Favicon.Host)
	require.Equal(t, "png", updated.Metadata.Favicon.Format)
	require.NotEmpty(t, updated.Metadata.Favicon.Data)
	require.Same(t, updated.Metadata.Favicon, NewFyneItemWidget(updated, nil).Icon())
	require.Equal(t, int32(2), atomic.LoadInt32(&requestCount))
}

// TestFyneItemWidgetEditsPasswordSettings checks password settings survive edits.
func TestFyneItemWidgetEditsPasswordSettings(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	settings := model.PasswordSettings{Mode: paw.PassphrasePassword, Length: 5, Format: paw.LowercaseFormat}
	item, err := model.NewPassword("Old title", "old password", settings)
	require.NoError(t, err)
	widget := NewFyneItemWidget(item, paw.NewDefaultPreferences()).(*modelItemWidget)
	widget.Edit(context.Background(), nil, app.NewWindow("Edit password"))
	widget.title.SetText("New title")
	widget.password.SetText("new password")
	updated, err := widget.OnSubmit()
	require.NoError(t, err)
	require.Equal(t, "New title", updated.Title)
	credential, _, err := widgetBasicAuth(updated)
	require.NoError(t, err)
	require.Equal(t, "new password", credential.Password.Value)
	got := uiPasswordSpec(credential.Password)
	require.Equal(t, settings.Mode, got.Mode)
	require.Equal(t, settings.Length, got.Length)
	require.Equal(t, settings.Format, got.Format)
}

// TestFyneItemWidgetEditsSSHCredentialRoundTrip checks the SSH credential survives editing.
func TestFyneItemWidgetEditsSSHCredentialRoundTrip(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	key, err := generateCXFSSHKey()
	require.NoError(t, err)
	key.KeyComment = strings.Repeat("long-comment-", 100)
	settings := model.PasswordSettings{Mode: paw.PassphrasePassword, Length: 5}
	item, err := model.NewSSHKey("Work key", key, "old passphrase", settings)
	require.NoError(t, err)
	itemWidget := NewFyneItemWidget(item, paw.NewDefaultPreferences()).(*modelItemWidget)
	view := itemWidget.Show(context.Background(), app.NewWindow("View SSH key"))
	viewObjects := test.LaidOutObjects(view)
	var viewHasMultilineEntry, viewHasScroll bool
	for _, object := range viewObjects {
		if entry, ok := object.(*widget.Entry); ok && entry.MultiLine {
			viewHasMultilineEntry = true
		}
		if _, ok := object.(*container.Scroll); ok {
			viewHasScroll = true
		}
	}
	require.True(t, viewHasMultilineEntry)
	require.True(t, viewHasScroll)
	require.Less(t, view.MinSize().Width, float32(600))
	form := itemWidget.Edit(context.Background(), nil, app.NewWindow("Edit SSH key"))
	require.Less(t, form.MinSize().Width, float32(600))
	formObjects := test.LaidOutObjects(form)
	var formHasMultilineEntry, formHasScroll bool
	for _, object := range formObjects {
		if entry, ok := object.(*widget.Entry); ok && entry.MultiLine {
			formHasMultilineEntry = true
		}
		if _, ok := object.(*container.Scroll); ok {
			formHasScroll = true
		}
	}
	require.True(t, formHasMultilineEntry)
	require.True(t, formHasScroll)
	itemWidget.title.SetText("Updated key")
	itemWidget.addToAgent.SetChecked(true)
	itemWidget.password.SetText("new passphrase")

	_, publicKey, fingerprint, err := displaySSHKey(key)
	require.NoError(t, err)
	require.Equal(t, publicKey, itemWidget.sshPublicKey.Text)
	require.Equal(t, fingerprint, itemWidget.sshFingerprint.Text)
	checkPrivateKey := func(text, expectedFingerprint string) {
		private, err := ssh.ParseRawPrivateKey([]byte(text))
		require.NoError(t, err)
		signer, err := ssh.NewSignerFromKey(private)
		require.NoError(t, err)
		require.Equal(t, expectedFingerprint, ssh.FingerprintSHA256(signer.PublicKey()))
	}
	checkPrivateKey(itemWidget.sshPrivateKey.Text, fingerprint)
	require.True(t, itemWidget.sshPrivateKey.Disabled())
	require.True(t, itemWidget.sshPrivateKey.MultiLine)
	var generateReplacement *widget.Button
	for _, object := range test.LaidOutObjects(form) {
		button, ok := object.(*widget.Button)
		if ok && button.Text == "Generate key" {
			generateReplacement = button
			break
		}
	}
	require.NotNil(t, generateReplacement)
	generateReplacement.OnTapped()
	require.NotEqual(t, fingerprint, itemWidget.sshFingerprint.Text)
	_, replacementPublicKey, replacementFingerprint, err := displaySSHKey(*itemWidget.sshSpec)
	require.NoError(t, err)
	require.Equal(t, replacementPublicKey, itemWidget.sshPublicKey.Text)
	require.Equal(t, replacementFingerprint, itemWidget.sshFingerprint.Text)
	checkPrivateKey(itemWidget.sshPrivateKey.Text, replacementFingerprint)

	updated, err := itemWidget.OnSubmit()
	require.NoError(t, err)
	require.Equal(t, "Updated key", updated.Title)
	require.True(t, updated.Metadata.AddToAgent)
	credential, err := uiSSHCredential(updated)
	require.NoError(t, err)
	_, _, fingerprint, err = displaySSHKey(credential)
	require.NoError(t, err)
	require.NotEmpty(t, fingerprint)
	password, _, err := widgetBasicAuth(updated)
	require.NoError(t, err)
	require.Equal(t, "new passphrase", password.Password.Value)
	gotSettings := uiPasswordSpec(password.Password)
	require.Equal(t, settings.Mode, gotSettings.Mode)
	require.Equal(t, settings.Length, gotSettings.Length)
}

// TestSSHViewsRevealKeysWithoutResizing checks the view and editor fit the current window.
func TestSSHViewsRevealKeysWithoutResizing(t *testing.T) {
	application := test.NewApp()
	t.Cleanup(application.Quit)
	key, err := generateCXFSSHKey()
	require.NoError(t, err)
	key.KeyComment = strings.Repeat("long-comment-", 100)
	item, err := model.NewSSHKey("Work key", key, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	for _, mode := range []string{"view", "edit"} {
		t.Run(mode, func(t *testing.T) {
			window := application.NewWindow("SSH key")
			t.Cleanup(window.Close)
			itemWidget := NewFyneItemWidget(item, paw.NewDefaultPreferences())
			var content fyne.CanvasObject
			if mode == "edit" {
				content = itemWidget.Edit(context.Background(), nil, window)
			} else {
				content = itemWidget.Show(context.Background(), window)
			}
			window.SetContent(content)
			window.Resize(fyne.NewSize(420, 540))
			initialWidth := window.Canvas().Size().Width
			var privateSection *widget.Accordion
			var reveal *widget.Button
			for _, object := range test.LaidOutObjects(content) {
				if accordion, ok := object.(*widget.Accordion); ok {
					privateSection = accordion
				}
				if button, ok := object.(*widget.Button); ok && button.Text == "Private key" {
					reveal = button
				}
			}
			require.NotNil(t, privateSection)
			require.False(t, privateSection.Items[0].Open)
			require.NotNil(t, reveal)
			test.Tap(reveal)
			require.True(t, privateSection.Items[0].Open)
			require.Equal(t, initialWidth, window.Canvas().Size().Width)
			var privateEntry *widget.Entry
			for _, object := range test.LaidOutObjects(privateSection) {
				if entry, ok := object.(*widget.Entry); ok {
					privateEntry = entry
				}
			}
			require.NotNil(t, privateEntry)
			require.Contains(t, privateEntry.Text, "BEGIN OPENSSH PRIVATE KEY")
			require.True(t, privateEntry.Disabled())
			require.Equal(t, fyne.TextWrapBreak, privateEntry.Wrapping)
			require.LessOrEqual(t, privateEntry.Size().Width, initialWidth)
			test.Tap(reveal)
			require.False(t, privateSection.Items[0].Open)
		})
	}
}

// TestFyneSSHEditorOpensPassphraseGenerator checks the SSH passphrase uses the password generator.
func TestFyneSSHEditorOpensPassphraseGenerator(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	vaultKey, err := paw.MakeOneTimeKey()
	require.NoError(t, err)
	sshCredential, err := generateCXFSSHKey()
	require.NoError(t, err)
	item, err := model.NewSSHKey("Work key", sshCredential, "", model.PasswordSettings{Mode: paw.CustomPassword})
	require.NoError(t, err)
	itemWidget := NewFyneItemWidget(item, paw.NewDefaultPreferences()).(*modelItemWidget)
	form := itemWidget.Edit(context.Background(), vaultKey, app.NewWindow("Edit SSH key"))

	var generatePassphrase *widget.Button
	for _, object := range test.LaidOutObjects(form) {
		button, ok := object.(*widget.Button)
		if ok && button.Text == "" && button.Icon == icon.KeyOutlinedIconThemed {
			generatePassphrase = button
			break
		}
	}
	require.NotNil(t, generatePassphrase)
	require.NotPanics(t, func() { generatePassphrase.OnTapped() })
}
