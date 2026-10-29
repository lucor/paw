// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
	"testing"
)

func entryListFixture() EntryListOptions {
	return EntryListOptions{VaultName: "Personal", VaultActions: NewVaultActions(func() {}, func() {}, func() {}), AddEntry: func(EntryKind) {}, AddVault: func() {}, Categories: []EntryCategory{
		{ID: "all", Label: "All items", Icon: icons.Vault}, {ID: "login", Label: "Logins", Icon: icons.Login},
		{ID: "password", Label: "Passwords", Icon: icons.Password}, {ID: "note", Label: "Notes", Icon: icons.Note}, {ID: "ssh", Label: "SSH keys", Icon: icons.SSH},
	}, Vaults: []VaultChoice{
		{Name: "Personal", Unlocked: true, Active: true, Open: func() {}}, {Name: "Work", Open: func() {}},
	}, Entries: []EntryRow{
		{Title: "GitHub", Subtitle: "dev@example.com", Category: "login", Icon: icons.Login, Open: func() {}},
		{Title: "Google", Subtitle: "user@gmail.com", Category: "login", Icon: icons.Login, Open: func() {}},
		{Title: "Home WiFi", Category: "password", Icon: icons.Password, Open: func() {}},
		{Title: "Production server", Subtitle: "deploy", Category: "ssh", Icon: icons.SSH, Open: func() {}},
		{Title: "Recovery codes", Category: "note", Icon: icons.Note, Open: func() {}},
	}}
}

func TestEntryListGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			assertViewGolden(t, "entry-list-"+variantName(variant)+".png", variant, fyne.NewSize(440, 650), func() fyne.CanvasObject { return NewEntryList(entryListFixture()) })
		})
	}
}

func TestEntryCategoryMenuGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewTempApp(t)
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			view := NewEntryList(entryListFixture())
			window := test.NewTempWindow(t, view)
			window.Resize(fyne.NewSize(440, 650))
			var label *widget.Label
			for _, object := range test.LaidOutObjects(view) {
				if candidate, ok := object.(*widget.Label); ok && candidate.Text == "All items" {
					label = candidate
					break
				}
			}
			require.NotNil(t, label)
			position := app.Driver().AbsolutePositionForObject(label).Add(fyne.NewPos(label.Size().Width/2, label.Size().Height/2))
			test.TapCanvas(window.Canvas(), position)
			test.AssertImageMatches(t, "entry-category-menu-"+variantName(variant)+".png", window.Canvas().Capture())
		})
	}
}

// TestEntryListSearchAndNavigation checks combined filters and row callbacks.
func TestEntryListSearchAndNavigation(t *testing.T) {
	test.NewTempApp(t)
	options := entryListFixture()
	opened := false
	options.Entries[0].Open = func() { opened = true }
	var savedSearch, savedCategory string
	options.OnFilterChanged = func(search, category string) { savedSearch, savedCategory = search, category }
	tabs := NewEntryList(options)
	window := test.NewTempWindow(t, tabs)
	window.Resize(fyne.NewSize(440, 650))
	var search *widget.Entry
	var list *widget.List
	var categoryLabel *widget.Label
	var count *widget.Label
	var navigation *Navigation
	for _, object := range test.LaidOutObjects(tabs) {
		if value, ok := object.(*Navigation); ok {
			navigation = value
		}
		switch value := object.(type) {
		case *widget.Entry:
			search = value
		case *widget.List:
			list = value
		case *widget.Button:
		case *widget.Label:
			if value.Text == "All items" {
				categoryLabel = value
			}
			if value.Text == "5 items" {
				count = value
			}
		}
	}
	require.NotNil(t, search)
	require.NotNil(t, list)
	require.NotNil(t, categoryLabel)
	require.NotNil(t, count)
	require.Equal(t, 5, list.Length())
	chooseEntryCategory(t, appDriverFor(window), window, categoryLabel, "Logins")
	require.Equal(t, 2, list.Length())
	require.Equal(t, "2 items", count.Text)
	search.SetText("GIT")
	require.Equal(t, 1, list.Length())
	list.Select(0)
	require.True(t, opened)
	require.Equal(t, "GIT", savedSearch)
	require.Equal(t, "login", savedCategory)
	search.SetText("deploy")
	require.Zero(t, list.Length())
	require.Equal(t, "0 items", count.Text)
	chooseEntryCategory(t, appDriverFor(window), window, categoryLabel, "All items")
	require.Equal(t, 1, list.Length(), "search includes the subtitle")
	require.NotNil(t, navigation)
	navigation.Select(1)
	var back *widget.Button
	foundCodes := false
	for _, object := range test.LaidOutObjects(tabs) {
		if label, ok := object.(*widget.Label); ok && label.Text == "Codes" {
			foundCodes = true
		}
		if button, ok := object.(*widget.Button); ok && button.Text == "Back" {
			back = button
		}
	}
	require.True(t, foundCodes)
	require.NotNil(t, back)
	test.Tap(back)
	require.Equal(t, 0, navigation.Selected)
	require.Equal(t, "deploy", search.Text)
}

// TestEntryListVaultMenu checks switching and adding a vault from the header.
func TestEntryListVaultMenu(t *testing.T) {
	app := test.NewTempApp(t)
	options := entryListFixture()
	switched, addedVault, addedEntry := false, false, false
	options.Vaults[1].Open = func() { switched = true }
	options.AddVault = func() { addedVault = true }
	options.AddEntry = func(kind EntryKind) { addedEntry = kind == EntryNote }
	view := NewEntryList(options)
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(440, 650))
	var name *widget.Label
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == "Personal" {
			name = label
		}
		if button, ok := object.(*iconButton); ok && button.Icon == icons.Plus {
			test.Tap(button)
			menu := window.Canvas().Overlays().Top()
			require.NotNil(t, menu)
			var addNote fyne.Tappable
			for _, child := range test.LaidOutObjects(menu) {
				control, ok := child.(fyne.Widget)
				if !ok {
					continue
				}
				for _, rendered := range test.WidgetRenderer(control).Objects() {
					if label, ok := rendered.(*canvas.Text); ok && label.Text == "Note" {
						addNote, _ = child.(fyne.Tappable)
					}
				}
			}
			require.NotNil(t, addNote)
			test.Tap(addNote)
		}
	}
	require.True(t, addedEntry)
	require.NotNil(t, name)
	for _, text := range []string{"Work — Locked", "Add vault"} {
		test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(name).Add(fyne.NewPos(name.Size().Width/2, name.Size().Height/2)))
		menu := window.Canvas().Overlays().Top()
		require.NotNil(t, menu)
		var item *widget.Label
		for _, object := range test.LaidOutObjects(menu) {
			if label, ok := object.(*widget.Label); ok && label.Text == text {
				item = label
			}
		}
		require.NotNil(t, item)
		test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(item).Add(fyne.NewPos(item.Size().Width/2, item.Size().Height/2)))
		require.Nil(t, window.Canvas().Overlays().Top())
	}
	require.True(t, switched)
	require.True(t, addedVault)
}

func chooseEntryCategory(t *testing.T, app fyne.App, window fyne.Window, label *widget.Label, name string) {
	t.Helper()
	position := app.Driver().AbsolutePositionForObject(label).Add(fyne.NewPos(label.Size().Width/2, label.Size().Height/2))
	test.TapCanvas(window.Canvas(), position)
	menu := window.Canvas().Overlays().Top()
	require.NotNil(t, menu)
	found := false
	for _, object := range test.LaidOutObjects(menu) {
		if label, ok := object.(*widget.Label); ok && label.Text == name {
			position := app.Driver().AbsolutePositionForObject(label).Add(fyne.NewPos(label.Size().Width/2, label.Size().Height/2))
			test.TapCanvas(window.Canvas(), position)
			found = true
			break
		}
	}
	require.True(t, found, "category label was not rendered")
}

func appDriverFor(window fyne.Window) fyne.App {
	return fyne.CurrentApp()
}
