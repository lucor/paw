// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"go.lucor.dev/paw/internal/favicon"
	"go.lucor.dev/paw/internal/paw"
)

type loginURLEntry struct {
	widget.Entry

	ctx             context.Context
	loginURL        *paw.LoginURL
	preferences     paw.FaviconDownloaderPreferences
	validationError error
	FaviconListener func(*paw.Favicon)
}

// newLoginURLEntry creates a Website field that fetches a favicon when its host changes.
func newLoginURLEntry(ctx context.Context, value string, preferences paw.FaviconDownloaderPreferences) *loginURLEntry {
	if ctx == nil {
		ctx = context.Background()
	}
	loginURL := paw.NewLoginURL()
	if value != "" {
		_ = loginURL.Set(value)
	}
	entry := &loginURLEntry{ctx: ctx, loginURL: loginURL, preferences: preferences}
	entry.ExtendBaseWidget(entry)
	entry.SetText(value)
	entry.Validator = func(string) error { return entry.validationError }
	entry.OnSubmitted = func(string) { entry.FocusLost() }
	return entry
}

// FocusGained clears a previous URL validation error.
func (e *loginURLEntry) FocusGained() {
	defer e.Entry.FocusGained()
	e.validationError = nil
}

// FocusLost normalizes the URL and fetches an icon when its host changes.
func (e *loginURLEntry) FocusLost() {
	defer e.Entry.FocusLost()
	oldHost := e.loginURL.URL().Hostname()
	if err := e.loginURL.Set(e.Text); err != nil {
		e.validationError = err
		return
	}
	e.validationError = nil
	if normalized := e.loginURL.String(); e.Text != normalized {
		e.SetText(normalized)
	}
	if oldHost != e.loginURL.URL().Hostname() {
		e.downloadFavicon()
	}
}

// downloadFavicon fetches the favicon without blocking the UI.
func (e *loginURLEntry) downloadFavicon() {
	host := e.loginURL.URL().Hostname()
	if host == "" || e.preferences.Disabled || e.loginURL.URL().Scheme != "https" {
		e.notifyFavicon(nil, host)
		return
	}
	pageURL := *e.loginURL.URL()
	go func() {
		data, format, err := favicon.Download(e.ctx, &pageURL, favicon.Options{})
		var downloaded *paw.Favicon
		if err == nil {
			downloaded = paw.NewFavicon(host, data, format)
		}
		e.notifyFavicon(downloaded, host)
	}()
}

// notifyFavicon applies a result only if the field still has the requested host.
func (e *loginURLEntry) notifyFavicon(downloaded *paw.Favicon, host string) {
	fyne.Do(func() {
		if e.loginURL.URL().Hostname() == host && e.FaviconListener != nil {
			e.FaviconListener(downloaded)
		}
	})
}
