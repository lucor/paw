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

// LoginURLEntry normalizes website URLs and fetches favicons after focus loss.
type LoginURLEntry struct {
	widget.Entry

	ctx             context.Context
	loginURL        *paw.LoginURL
	settings        paw.FaviconDownloaderSettings
	FaviconListener func(*paw.Favicon)
}

// NewLoginURLEntry creates a Website field that fetches a favicon when its host changes.
func NewLoginURLEntry(ctx context.Context, value string, settings paw.FaviconDownloaderSettings) *LoginURLEntry {
	if ctx == nil {
		ctx = context.Background()
	}
	loginURL := paw.NewLoginURL()
	if value != "" {
		_ = loginURL.Set(value)
	}
	entry := &LoginURLEntry{ctx: ctx, loginURL: loginURL, settings: settings}
	entry.ExtendBaseWidget(entry)
	entry.SetText(value)
	entry.OnSubmitted = func(string) { entry.FocusLost() }
	return entry
}

// FocusLost normalizes the URL and fetches an icon when its host changes.
func (e *LoginURLEntry) FocusLost() {
	defer e.Entry.FocusLost()
	oldHost := e.loginURL.URL().Hostname()
	if err := e.loginURL.Set(e.Text); err != nil {
		e.notifyFavicon(nil, "")
		return
	}
	if normalized := e.loginURL.String(); e.Text != normalized {
		e.SetText(normalized)
	}
	if oldHost != e.loginURL.URL().Hostname() {
		e.notifyFavicon(nil, e.loginURL.URL().Hostname())
		e.downloadFavicon()
	}
}

func (e *LoginURLEntry) downloadFavicon() {
	host := e.loginURL.URL().Hostname()
	if host == "" || e.settings.Disabled || e.loginURL.URL().Scheme != "https" {
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

func (e *LoginURLEntry) notifyFavicon(downloaded *paw.Favicon, host string) {
	fyne.Do(func() {
		if host == "" && downloaded == nil && e.FaviconListener != nil {
			currentURL := paw.NewLoginURL()
			if e.Text == "" || currentURL.Set(e.Text) != nil {
				e.FaviconListener(nil)
			}
			return
		}
		currentURL := paw.NewLoginURL()
		if e.Text != "" && currentURL.Set(e.Text) != nil {
			return
		}
		if currentURL.URL().Hostname() == host && e.FaviconListener != nil {
			e.FaviconListener(downloaded)
		}
	})
}
