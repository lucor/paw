// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

const (
	aboutPageTitle          = "About Paw"
	aboutLicenseDescription = "Paw is free and open-source software, licensed under the GNU Affero General Public License v3.0 or later."
	aboutWebsiteURL         = "https://paw.pm"
	aboutLicenseURL         = "https://github.com/lucor/paw/blob/main/LICENSE"
	aboutThirdPartyURL      = "https://github.com/lucor/paw/blob/main/THIRD_PARTY_LICENSES"
	aboutSourceURL          = "https://github.com/lucor/paw"
	aboutIssueURL           = "https://github.com/lucor/paw/issues"
)

// NewAboutPawView builds the About page with Paw's shared secondary header and navigation rows.
func NewAboutPawView(window fyne.Window, back func()) fyne.CanvasObject {
	return newAboutPawView(back, func(target *url.URL) {
		if err := fyne.CurrentApp().OpenURL(target); err != nil {
			dialog.ShowError(err, window)
		}
	})
}

func (a *app) showAboutPawView(back fyne.CanvasObject) {
	a.clearUnlockPassword()
	if back == nil {
		back = a.main
	}
	a.win.SetContent(NewAboutPawView(a.win, func() { a.win.SetContent(back) }))
}

func newAboutPawView(back func(), openURL func(*url.URL)) fyne.CanvasObject {
	backButton := newIconButton("Back", icons.Back, back)
	backButton.Importance = widget.LowImportance
	header := newPageHeader(backButton, newText(aboutPageTitle, titleStyle), nil)

	logo := container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(64), widget.NewIcon(icons.Paw)))
	name := newText("Paw", bodyStyle)
	name.TextStyle.Bold = true
	name.Alignment = fyne.TextAlignCenter
	version := newText("Version "+paw.Version(), captionStyle)
	version.Alignment = fyne.TextAlignCenter
	hero := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNameInnerPadding)), logo, name, version)
	description := newText(aboutLicenseDescription, bodyStyle)
	links := aboutLinkRows(openURL)
	content := container.New(layout.NewCustomPaddedVBoxLayout(theme.Size(theme.SizeNamePadding)),
		hero,
		description,
		widget.NewSeparator(),
		links,
	)
	return newSecondaryPage(header, nil, container.NewVScroll(content))
}

func aboutLinkRows(openURL func(*url.URL)) fyne.CanvasObject {
	links := []struct {
		label, target string
		icon          fyne.Resource
	}{
		{"Website", aboutWebsiteURL, icons.Globe},
		{"License", aboutLicenseURL, icons.Scale},
		{"Third-party licenses", aboutThirdPartyURL, icons.ScrollText},
		{"Source code", aboutSourceURL, icons.Code},
		{"Report an issue", aboutIssueURL, icons.Bug},
	}
	rows := make([]fyne.CanvasObject, 0, len(links)*2-1)
	for index, link := range links {
		if index > 0 {
			rows = append(rows, widget.NewSeparator())
		}
		target, _ := url.Parse(link.target)
		leading := container.NewGridWrap(fyne.NewSquareSize(40), container.NewCenter(container.NewGridWrap(fyne.NewSquareSize(20), widget.NewIcon(link.icon))))
		rows = append(rows, newNavigationRowWithTrailingIcon(link.label, leading, icons.External, func() { openURL(target) }))
	}
	return container.NewVBox(rows...)
}
