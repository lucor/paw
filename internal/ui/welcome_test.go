// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// TestWelcomeActionUsesWholeRow checks taps and hover through the row's content.
func TestWelcomeActionUsesWholeRow(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Welcome")
	t.Cleanup(window.Close)

	for _, appearance := range []fyne.Theme{variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantLight}, variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantDark}} {
		app.Settings().SetTheme(appearance)
		calls := 0
		row := welcomeAction("Create new vault", "Start with an empty encrypted vault", icons.CreateVault, func() { calls++ })
		window.SetContent(row)
		window.Resize(fyne.NewSize(440, 100))
		window.Show()
		objects := test.LaidOutObjects(row)
		leftIcon := findIcon(t, objects, icons.CreateVault.Name())
		require.Equal(t, fyne.NewSquareSize(32), leftIcon.Size(), "left action icon size")
		require.Equal(t, theme.Color(theme.ColorNameForeground), renderedTextColor(t, objects, "Create new vault"))
		require.Equal(t, theme.Color(theme.ColorNameForeground), renderedTextColor(t, objects, "Start with an empty encrypted vault"))

		// Labels and both icons must pass their clicks to the button underneath.
		for _, object := range test.LaidOutObjects(row) {
			switch object.(type) {
			case *widget.Label, *widget.Icon:
				position := app.Driver().AbsolutePositionForObject(object).Add(fyne.NewPos(object.Size().Width/2, object.Size().Height/2))
				test.TapCanvas(window.Canvas(), position)
			}
		}
		require.Equal(t, 4, calls, "title, subtitle, icon and arrow must all activate the row")

		button := navigationRowButton(t, row)
		background := test.WidgetRenderer(button).Objects()[0].(*canvas.Rectangle)
		normal := background.FillColor
		test.MoveMouse(window.Canvas(), app.Driver().AbsolutePositionForObject(row).Add(fyne.NewPos(2, 2)))
		require.NotEqual(t, normal, background.FillColor, "hover must highlight the underlying button")
		require.Equal(t, row.Size(), background.Size(), "highlight must cover the whole row")
	}
}

func TestAddVaultKeepsRestoreAction(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Add vault")
	t.Cleanup(window.Close)
	called := false
	view := NewAddVault(func() {}, func() {}, func() { called = true }, func() {})
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 300))
	window.Show()

	var restoreTitle *widget.Label
	var restoreIcon bool
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && label.Text == "Restore from backup" {
			restoreTitle = label
		}
		if icon, ok := object.(*widget.Icon); ok && icon.Resource == icons.ArchiveRestore {
			restoreIcon = true
		}
	}
	require.NotNil(t, restoreTitle)
	require.True(t, restoreIcon)
	test.TapCanvas(window.Canvas(), app.Driver().AbsolutePositionForObject(restoreTitle).Add(fyne.NewPos(restoreTitle.Size().Width/2, restoreTitle.Size().Height/2)))
	require.True(t, called)
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok {
			require.NotEqual(t, "Connect via Sync", label.Text)
		}
	}
}

func TestAddVaultDescriptionsWrapAtNarrowWidth(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Add vault")
	t.Cleanup(window.Close)
	view := NewAddVault(func() {}, func() {}, func() {}, func() {})
	window.SetContent(view)
	window.Resize(fyne.NewSize(320, 540))
	window.Show()
	for _, object := range test.LaidOutObjects(view) {
		if label, ok := object.(*widget.Label); ok && (label.Text == "Start with an empty encrypted vault" || label.Text == "Create a vault from a backup file") {
			require.Equal(t, fyne.TextWrapWord, label.Wrapping)
			require.Equal(t, fyne.TextTruncateOff, label.Truncation)
			require.Greater(t, label.Size().Height, float32(0))
		}
	}
}

// TestWelcomeRowSpacing checks the gaps between the welcome actions.
func TestWelcomeRowSpacing(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Welcome")
	t.Cleanup(window.Close)
	content := NewWelcome(widget.NewLabel("Paw"), WelcomeActions{})
	window.SetContent(content)
	window.Show()
	rows := welcomeRows(t, content)
	require.Len(t, rows, 3)
	for i := 1; i < len(rows); i++ {
		gap := rows[i].Position().Y - rows[i-1].Position().Y - rows[i-1].Size().Height
		require.InDelta(t, theme.Size(theme.SizeNameInnerPadding), gap, 0.01)
	}
}

// TestWelcomeRowsExpandWithWindow checks action rows use the available width.
func TestWelcomeRowsExpandWithWindow(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Welcome")
	t.Cleanup(window.Close)
	window.SetContent(NewWelcome(widget.NewLabel("Paw"), WelcomeActions{}))
	window.Resize(fyne.NewSize(440, 700))
	window.Show()
	narrowWidth := welcomeRowWidth(t, window.Content())
	window.Resize(fyne.NewSize(800, 700))
	wideWidth := welcomeRowWidth(t, window.Content())
	require.Greater(t, wideWidth, narrowWidth)
}

func welcomeRowWidth(t *testing.T, content fyne.CanvasObject) float32 {
	t.Helper()
	rows := welcomeRows(t, content)
	return rows[0].Size().Width
}

// TestWelcomeFrameFollowsThemeChanges checks an existing frame updates its border.
func TestWelcomeFrameFollowsThemeChanges(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	row := welcomeAction("Restore from backup", "Open an existing vault file", icons.RestoreVault, func() {})
	window := app.NewWindow("Welcome")
	t.Cleanup(window.Close)
	window.SetContent(row)
	window.Show()
	for _, appearance := range []fyne.Theme{
		variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantLight},
		variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantDark},
		variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantLight},
	} {
		app.Settings().SetTheme(appearance)
		row.Refresh()
		button := navigationRowButton(t, row)
		require.NotNil(t, button)
	}
}

func welcomeRows(t *testing.T, content fyne.CanvasObject) []*fyne.Container {
	t.Helper()
	var rows []*fyne.Container
	for _, object := range test.LaidOutObjects(content) {
		if row, ok := object.(*fyne.Container); ok && len(row.Objects) == 2 {
			if _, ok := row.Objects[0].(*widget.Button); ok {
				rows = append(rows, row)
			}
		}
	}
	return rows
}

func navigationRowButton(t *testing.T, row fyne.CanvasObject) *widget.Button {
	t.Helper()
	container, ok := row.(*fyne.Container)
	require.True(t, ok)
	button, ok := container.Objects[0].(*widget.Button)
	require.True(t, ok)
	return button
}

type variantTheme struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (t variantTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return t.Theme.Color(name, t.variant)
}

func findIcon(t *testing.T, objects []fyne.CanvasObject, name string) *widget.Icon {
	t.Helper()
	for _, object := range objects {
		if icon, ok := object.(*widget.Icon); ok && icon.Resource.Name() == name {
			return icon
		}
	}
	t.Fatalf("icon %q was not rendered", name)
	return nil
}

func renderedTextColor(t *testing.T, objects []fyne.CanvasObject, text string) color.Color {
	t.Helper()
	for _, object := range objects {
		if rendered, ok := object.(*canvas.Text); ok && rendered.Text == text {
			return rendered.Color
		}
	}
	t.Fatalf("text %q was not rendered", text)
	return nil
}
