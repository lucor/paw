// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestCompactTextColorsFollowApplicationTheme(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	applySettingsTheme(model.ThemeSystem)
	compact := compactTextTheme{Theme: app.Settings().Theme()}
	for _, selection := range []string{model.ThemeSystem, model.ThemeDark, model.ThemeLight} {
		applySettingsTheme(selection)
		variant := app.Settings().ThemeVariant()
		require.Equal(t, app.Settings().Theme().Color(theme.ColorNameForeground, variant), compact.Color(theme.ColorNameForeground, variant), selection)
	}
}

// TestTypographyFollowsTheme checks rendered roles keep their hierarchy across theme changes.
func TestTypographyFollowsTheme(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Typography")
	t.Cleanup(window.Close)
	cases := []struct {
		text  string
		style textStyle
		size  fyne.ThemeSizeName
		bold  bool
		color fyne.ThemeColorName
	}{
		{"Title", titleStyle, theme.SizeNameSubHeadingText, true, theme.ColorNameForeground},
		{"Body", bodyStyle, theme.SizeNameText, false, theme.ColorNameForeground},
		{"Caption", captionStyle, theme.SizeNameCaptionText, false, theme.ColorNameForeground},
		{"Heading", headingStyle, theme.SizeNameHeadingText, true, theme.ColorNameForeground},
	}
	content := container.NewVBox()
	for _, item := range cases {
		content.Add(newText(item.text, item.style))
	}
	window.SetContent(content)
	window.Resize(fyne.NewSize(440, 700))
	window.Show()
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark, theme.VariantLight} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
		for _, object := range content.Objects {
			object.Refresh()
		}
		errorLabel := newErrorText("Error")
		content.Add(errorLabel)
		content.Refresh()
		objects := test.LaidOutObjects(content)
		for _, item := range cases {
			var rendered *canvas.Text
			for _, object := range objects {
				if text, ok := object.(*canvas.Text); ok && text.Text == item.text {
					rendered = text
					break
				}
			}
			require.NotNil(t, rendered, item.text)
			require.Equal(t, theme.Size(item.size), rendered.TextSize, item.text)
			require.Equal(t, item.bold, rendered.TextStyle.Bold, item.text)
			require.Equal(t, theme.Color(item.color), rendered.Color, item.text)
		}
		var errorText *canvas.Text
		for _, object := range objects {
			if text, ok := object.(*canvas.Text); ok && text.Text == "Error" {
				errorText = text
				break
			}
		}
		require.NotNil(t, errorText)
		require.Equal(t, theme.Size(theme.SizeNameCaptionText), errorText.TextSize)
		require.Equal(t, theme.Color(theme.ColorNameError), errorText.Color)
		content.Remove(errorLabel)
	}
}
