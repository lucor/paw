// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"image/color"
	"math"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestPawThemeSemanticColors(t *testing.T) {
	cases := []struct {
		name  fyne.ThemeColorName
		light color.NRGBA
		dark  color.NRGBA
	}{
		{theme.ColorNameForeground, color.NRGBA{R: 0x25, G: 0x28, B: 0x2E, A: 0xFF}, color.NRGBA{R: 0xF1, G: 0xF2, B: 0xF4, A: 0xFF}},
		{theme.ColorNameDisabled, color.NRGBA{R: 0x68, G: 0x70, B: 0x7A, A: 0xFF}, color.NRGBA{R: 0xA9, G: 0xAD, B: 0xB5, A: 0xFF}},
		{theme.ColorNamePlaceHolder, color.NRGBA{R: 0x73, G: 0x7A, B: 0x84, A: 0xFF}, color.NRGBA{R: 0x92, G: 0x98, B: 0xA3, A: 0xFF}},
		{theme.ColorNameInputBorder, color.NRGBA{R: 0x85, G: 0x8B, B: 0x95, A: 0xFF}, color.NRGBA{R: 0x76, G: 0x7D, B: 0x89, A: 0xFF}},
		{theme.ColorNameDisabledButton, color.NRGBA{R: 0xE6, G: 0xE9, B: 0xED, A: 0xFF}, color.NRGBA{R: 0x33, G: 0x36, B: 0x3D, A: 0xFF}},
		{theme.ColorNameError, color.NRGBA{R: 0xB4, G: 0x23, B: 0x18, A: 0xFF}, color.NRGBA{R: 0xFF, G: 0x85, B: 0x85, A: 0xFF}},
	}
	pawTheme := newPawTheme()
	for _, item := range cases {
		t.Run(string(item.name), func(t *testing.T) {
			require.Equal(t, item.light, pawTheme.Color(item.name, theme.VariantLight))
			require.Equal(t, item.dark, pawTheme.Color(item.name, theme.VariantDark))
		})
	}
}

func TestPawThemeSharedSizes(t *testing.T) {
	app := test.NewTempApp(t)
	pawTheme := newPawTheme()
	app.Settings().SetTheme(pawTheme)
	basePadding := pawTheme.Theme.Size(theme.SizeNamePadding)
	for name, expected := range map[fyne.ThemeSizeName]float32{
		pawSizeNamePagePadding:    basePadding * 6,
		pawSizeNameFormSpacing:    basePadding * 4,
		pawSizeNameListIcon:       basePadding * 4,
		pawSizeNameListIconFrame:  basePadding * 8,
		pawSizeNameListIconArea:   basePadding * 10,
		pawSizeNameDetailIcon:     basePadding * 6,
		pawSizeNameDetailIconArea: basePadding * 8,
	} {
		require.Equal(t, expected, pawTheme.Size(name), string(name))
	}
	require.Equal(t, pawTheme.Theme.Size(theme.SizeNameInputBorder), pawTheme.Size(theme.SizeNameInputBorder))
}

func TestPawThemeDelegatesUnspecifiedColors(t *testing.T) {
	base := theme.DefaultTheme()
	pawTheme := newPawTheme()
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		for _, name := range []fyne.ThemeColorName{theme.ColorNameBackground, theme.ColorNamePrimary, theme.ColorNameSeparator} {
			require.Equal(t, base.Color(name, variant), pawTheme.Color(name, variant), string(name))
		}
	}
}

func TestPawThemeInstallationAndAppearanceChanges(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	installPawTheme(model.ThemeSystem)
	installed, ok := app.Settings().Theme().(*pawTheme)
	require.True(t, ok)

	applySettingsTheme(model.ThemeDark)
	require.Same(t, installed, app.Settings().Theme())
	require.Equal(t, color.NRGBA{R: 0xF1, G: 0xF2, B: 0xF4, A: 0xFF}, app.Settings().Theme().Color(theme.ColorNameForeground, theme.VariantLight))
	applySettingsTheme(model.ThemeLight)
	require.Same(t, installed, app.Settings().Theme())
	require.Equal(t, color.NRGBA{R: 0x25, G: 0x28, B: 0x2E, A: 0xFF}, app.Settings().Theme().Color(theme.ColorNameForeground, theme.VariantDark))
	applySettingsTheme(model.ThemeSystem)
	require.Same(t, installed, app.Settings().Theme())
	require.Equal(t, color.NRGBA{R: 0x25, G: 0x28, B: 0x2E, A: 0xFF}, app.Settings().Theme().Color(theme.ColorNameForeground, theme.VariantLight))
	require.Equal(t, color.NRGBA{R: 0xF1, G: 0xF2, B: 0xF4, A: 0xFF}, app.Settings().Theme().Color(theme.ColorNameForeground, theme.VariantDark))
}

func TestPawThemeTextAndControlContrast(t *testing.T) {
	pawTheme := newPawTheme()
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		background := pawTheme.Theme.Color(theme.ColorNameBackground, variant)
		inputBackground := pawTheme.Theme.Color(theme.ColorNameInputBackground, variant)
		require.GreaterOrEqual(t, contrastRatio(pawTheme.Color(theme.ColorNameForeground, variant), background), 4.5, "foreground on background")
		require.GreaterOrEqual(t, contrastRatio(pawTheme.Color(theme.ColorNameDisabled, variant), background), 4.5, "secondary text on background")
		require.GreaterOrEqual(t, contrastRatio(pawTheme.Color(theme.ColorNameError, variant), background), 4.5, "error on background")
		require.GreaterOrEqual(t, contrastRatio(pawTheme.Color(theme.ColorNameInputBorder, variant), inputBackground), 3.0, "input border")
		// The approved light placeholder color is below 4.5:1 on Fyne's input surface.
		require.GreaterOrEqual(t, contrastRatio(pawTheme.Color(theme.ColorNamePlaceHolder, variant), inputBackground), 3.0, "placeholder on input background")
	}
}

func contrastRatio(first, second color.Color) float64 {
	return (max(relativeLuminance(first), relativeLuminance(second)) + 0.05) / (min(relativeLuminance(first), relativeLuminance(second)) + 0.05)
}

func relativeLuminance(value color.Color) float64 {
	r, g, b, _ := value.RGBA()
	linear := func(component uint32) float64 {
		channel := float64(component) / 65535
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

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

func TestVaultItemListRefreshesWhenThemeChanges(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	installPawTheme(model.ThemeDark)
	window := app.NewWindow("Vault")
	t.Cleanup(window.Close)
	window.SetContent(NewEntryList(EntryListOptions{
		VaultName: "Personal",
		Entries:   []EntryRow{{Title: "GitHub", Subtitle: "dev@example.com", Category: "login", Icon: theme.InfoIcon()}},
	}))
	window.Resize(fyne.NewSize(440, 650))
	window.Show()

	assertEntryTitleColor := func(expected color.Color) {
		t.Helper()
		var got color.Color
		fyne.DoAndWait(func() {
			for _, object := range test.LaidOutObjects(window.Content()) {
				if text, ok := object.(*canvas.Text); ok && text.Text == "GitHub" {
					got = text.Color
					break
				}
			}
		})
		require.Equal(t, expected, got)
	}

	assertEntryTitleColor(color.NRGBA{R: 0xF1, G: 0xF2, B: 0xF4, A: 0xFF})
	applySettingsTheme(model.ThemeLight)
	assertEntryTitleColor(color.NRGBA{R: 0x25, G: 0x28, B: 0x2E, A: 0xFF})
	applySettingsTheme(model.ThemeDark)
	assertEntryTitleColor(color.NRGBA{R: 0xF1, G: 0xF2, B: 0xF4, A: 0xFF})
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

func TestSharedRowsUseSameTitleAndSecondaryTypography(t *testing.T) {
	app := test.NewTempApp(t)
	installPawTheme(model.ThemeSystem)
	window := app.NewWindow("Rows")
	t.Cleanup(window.Close)
	action := NewActionRow("Item", "Secondary", icons.Globe, nil)
	navigation := newNavigationRowWithSubtitle("Item", "Secondary", navigationRowIcon(icons.Globe), nil)
	window.SetContent(container.NewVBox(action, navigation))
	window.Show()

	var navigationLabels []*widget.Label
	fyne.DoAndWait(func() {
		for _, object := range test.LaidOutObjects(navigation) {
			if label, ok := object.(*widget.Label); ok {
				navigationLabels = append(navigationLabels, label)
			}
		}
	})
	var navTitle, navSecondary *widget.Label
	for _, label := range navigationLabels {
		switch label.Text {
		case "Item":
			navTitle = label
		case "Secondary":
			navSecondary = label
		}
	}
	require.NotNil(t, navTitle)
	require.NotNil(t, navSecondary)
	require.Equal(t, action.title.SizeName, navTitle.SizeName)
	require.Equal(t, action.title.TextStyle, navTitle.TextStyle)
	require.Equal(t, action.detail.SizeName, navSecondary.SizeName)
	require.Equal(t, action.detail.TextStyle, navSecondary.TextStyle)
}
