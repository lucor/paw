// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package v2

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/ui/v2/assets/icons"
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

		button := row.(*ActionRow).button
		background := test.WidgetRenderer(button).Objects()[0].(*canvas.Rectangle)
		normal := background.FillColor
		test.MoveMouse(window.Canvas(), app.Driver().AbsolutePositionForObject(row).Add(fyne.NewPos(2, 2)))
		require.NotEqual(t, normal, background.FillColor, "hover must highlight the underlying button")
		require.Equal(t, row.Size(), background.Size(), "highlight must cover the whole row")
		renderer := test.WidgetRenderer(row.(*ActionRow)).(*actionRowRenderer)
		require.Positive(t, renderer.outline.CornerRadius, "outline must have rounded corners")
		require.Positive(t, renderer.outline.StrokeWidth, "outline must remain visible without hover")
		require.Equal(t, row.Size(), renderer.outline.Size(), "outline must surround the whole row")
		require.Equal(t, theme.Color(theme.ColorNameInputBorder), renderer.outline.StrokeColor)
		position := app.Driver().AbsolutePositionForObject(leftIcon).Subtract(app.Driver().AbsolutePositionForObject(row))
		require.GreaterOrEqual(t, position.X, theme.Size(theme.SizeNameInnerPadding), "icon must have an inner margin")
		require.GreaterOrEqual(t, position.Y, theme.Size(theme.SizeNameInnerPadding), "icon must have a vertical margin")
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
	var rows []*ActionRow
	for _, object := range test.LaidOutObjects(content) {
		if row, ok := object.(*ActionRow); ok {
			rows = append(rows, row)
		}
	}
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
	for _, object := range test.LaidOutObjects(content) {
		if row, ok := object.(*ActionRow); ok {
			return row.Size().Width
		}
	}
	t.Fatal("welcome action row was not laid out")
	return 0
}

// TestWelcomeFrameFollowsThemeChanges checks an existing frame updates its border.
func TestWelcomeFrameFollowsThemeChanges(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	row := welcomeAction("Restore from backup", "Open an existing vault file", icons.RestoreVault, func() {}).(*ActionRow)
	window := app.NewWindow("Welcome")
	t.Cleanup(window.Close)
	window.SetContent(row)
	window.Show()
	renderer := test.WidgetRenderer(row).(*actionRowRenderer)

	for _, appearance := range []fyne.Theme{
		variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantLight},
		variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantDark},
		variantTheme{Theme: theme.DefaultTheme(), variant: theme.VariantLight},
	} {
		app.Settings().SetTheme(appearance)
		row.Refresh()
		require.Equal(t, theme.Color(theme.ColorNameInputBorder), renderer.outline.StrokeColor)
		background := test.WidgetRenderer(row.button).Objects()[0].(*canvas.Rectangle)
		require.Equal(t, background.CornerRadius, renderer.outline.CornerRadius, "outline must match the native hover surface")
		require.Equal(t, theme.Size(theme.SizeNameInputBorder), renderer.outline.StrokeWidth)
	}
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
