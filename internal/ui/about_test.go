// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"net/url"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestSettingsAboutNavigationReturnsToSameView(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Settings")
	t.Cleanup(window.Close)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	state := &model.AppState{Settings: model.NewDefaultSettings()}
	a := &app{win: window, main: container.NewVScroll(widget.NewLabel("Main")), state: state, storage: store}
	settings := a.makeSettingsView()
	window.SetContent(settings)
	window.Resize(fyne.NewSize(440, 800))
	window.Show()

	settingsObjects := test.LaidOutObjects(settings)
	info := findIcon(t, settingsObjects, icons.Info.Name())
	chevron := findIcon(t, settingsObjects, icons.Next.Name())
	require.Equal(t, fyne.NewSquareSize(pawSize(pawSizeNameListIcon)), info.Size())
	require.Equal(t, fyne.NewSquareSize(pawSize(pawSizeNameListIcon)), chevron.Size())
	test.Tap(findNavigationRowButton(t, settings, "About Paw"))
	about := window.Content()
	require.Contains(t, auditTexts(about), aboutPageTitle)
	for _, object := range test.LaidOutObjects(about) {
		_, hasNavigation := object.(*Navigation)
		require.False(t, hasNavigation, "secondary About view should hide bottom navigation")
	}

	for _, object := range test.LaidOutObjects(about) {
		if button, ok := object.(*iconButton); ok && button.Icon == icons.Back {
			test.Tap(button)
			break
		}
	}
	require.Same(t, settings, window.Content())
}

func TestAboutPawShowsLogoVersionAndExternalNavigationRows(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow(aboutPageTitle)
	t.Cleanup(window.Close)
	opened := ""
	view := newAboutPawView(func() {}, func(target *url.URL) { opened = target.String() })
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 800))
	window.Show()

	objects := test.LaidOutObjects(view)
	texts := auditTexts(view)
	require.Contains(t, texts, aboutPageTitle)
	require.Contains(t, texts, "Paw")
	require.Contains(t, texts, "Version "+paw.Version())
	require.Contains(t, texts, aboutLicenseDescription)
	logo := findIcon(t, objects, icons.Paw.Name())
	require.Equal(t, fyne.NewSquareSize(64), logo.Size())

	name, version := aboutTextLabels(t, objects)
	require.Equal(t, fyne.TextAlignCenter, name.Alignment)
	require.Equal(t, fyne.TextAlignCenter, version.Alignment)

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
	for _, link := range links {
		row := findNavigationRowButton(t, view, link.label)
		leading := findIcon(t, objects, link.icon.Name())
		require.Equal(t, fyne.NewSquareSize(pawSize(pawSizeNameListIcon)), leading.Size(), link.label)
		test.Tap(row)
		require.Equal(t, link.target, opened)
	}
	externalCount := 0
	for _, object := range objects {
		icon, ok := object.(*widget.Icon)
		if ok && icon.Resource == icons.External {
			externalCount++
			require.Equal(t, fyne.NewSquareSize(pawSize(pawSizeNameListIcon)), icon.Size())
		}
	}
	require.Equal(t, len(links), externalCount)
}

func aboutTextLabels(t *testing.T, objects []fyne.CanvasObject) (*widget.Label, *widget.Label) {
	t.Helper()
	var name, version *widget.Label
	for _, object := range objects {
		label, ok := object.(*widget.Label)
		if !ok {
			continue
		}
		switch label.Text {
		case "Paw":
			name = label
		case "Version " + paw.Version():
			version = label
		}
	}
	require.NotNil(t, name)
	require.NotNil(t, version)
	return name, version
}
