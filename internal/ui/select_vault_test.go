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
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestVaultAppearanceIsShownInVaultHeaderAndUnlock(t *testing.T) {
	test.NewTempApp(t)
	appearance := model.VaultAppearance{Icon: "briefcase-business", Color: "green"}
	options := entryListFixture()
	options.VaultAppearance = appearance
	window := test.NewTempWindow(t, NewEntryList(options))
	window.Resize(fyne.NewSize(440, 650))
	require.True(t, hasVaultAppearance(window.Content(), appearance))

	info := model.VaultInfo{ID: "vault-id", Name: "Work", Appearance: appearance}
	window.SetContent(NewUnlockVault(info, nil, nil, func() {}, func() {}, nil))
	require.True(t, hasVaultAppearance(window.Content(), appearance))
}

func TestVaultSelectorListUsesFullBoundsForKeyboardHighlight(t *testing.T) {
	test.NewTempApp(t)
	view := NewSelectVault([]VaultChoice{{Name: "Personal", Open: func() {}}}, func() {})
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(440, 650))

	var list *widget.List
	for _, object := range test.LaidOutObjects(view) {
		if candidate, ok := object.(*widget.List); ok {
			list = candidate
			break
		}
	}
	require.NotNil(t, list)
	_, ok := list.CreateItem().(*ActionRow)
	require.True(t, ok, "the outlined action row should fill the list item focus bounds")
}

func TestVaultAppearanceCanSelectNoColor(t *testing.T) {
	test.NewTempApp(t)
	var selected model.VaultAppearance
	view := newVaultAppearanceView("Work", model.VaultAppearance{Icon: "paw", Color: "green"}, func(appearance model.VaultAppearance) {
		selected = appearance
	}, func() {})
	var noColor *widget.Button
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Icon == nil && button.Text == "" {
			noColor = button
			break
		}
	}
	require.NotNil(t, noColor, "the first color option should be the no-color choice")
	test.Tap(noColor)
	require.Equal(t, model.VaultAppearance{Icon: "paw", Color: "none"}, selected)
	require.Equal(t, "none", model.DefaultVaultAppearance().Color)
	require.Equal(t, "paw", model.DefaultVaultAppearance().Icon)
	require.Len(t, vaultIcons, 8)
	require.Equal(t, "paw", vaultIcons[0].id)
	require.Same(t, icons.Paw, vaultIcons[0].resource)
}

func TestVaultAppearanceSwatchesMatchPreviewColors(t *testing.T) {
	app := test.NewTempApp(t)
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
		view := newVaultAppearanceView("Work", model.VaultAppearance{Icon: "paw", Color: "blue"}, nil, func() {})
		want := vaultIconPalette("blue", variant, app.Settings().Theme()).background
		var found bool
		for _, object := range test.LaidOutObjects(view) {
			circle, ok := object.(*canvas.Circle)
			if ok && circle.FillColor == want {
				found = true
				break
			}
		}
		require.True(t, found, "%s selector swatch should use the preview background color", variant)

		preview := newVaultIcon(model.VaultAppearance{Icon: "paw", Color: "blue"}, vaultIconMedium)
		renderer := test.WidgetRenderer(preview).(*vaultIconRenderer)
		require.Equal(t, want, renderer.background.FillColor)
	}
}

func TestVaultAppearancePreviewHasCompactGapOnNarrowWindow(t *testing.T) {
	test.NewTempApp(t)
	view := newVaultAppearanceView("Personal", model.DefaultVaultAppearance(), nil, func() {})
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(320, 540))
	var previewIcon *vaultIcon
	var previewName *widget.Label
	for _, object := range test.LaidOutObjects(view) {
		switch object := object.(type) {
		case *vaultIcon:
			if object.size == vaultIconMedium {
				previewIcon = object
			}
		case *widget.Label:
			if object.Text == "Personal" {
				previewName = object
			}
		}
	}
	require.NotNil(t, previewIcon)
	require.NotNil(t, previewName)
	iconPosition := fyne.CurrentApp().Driver().AbsolutePositionForObject(previewIcon)
	namePosition := fyne.CurrentApp().Driver().AbsolutePositionForObject(previewName)
	require.LessOrEqual(t, namePosition.X-(iconPosition.X+previewIcon.Size().Width), float32(4))
	require.LessOrEqual(t, namePosition.X+previewName.Size().Width, float32(296))
}

func TestSecondaryViewsShareHeaderPosition(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	window.Resize(fyne.NewSize(440, 800))
	views := []struct {
		title string
		view  fyne.CanvasObject
	}{
		{title: "Add vault", view: NewAddVault(func() {}, func() {}, func() {})},
		{title: "Create new vault", view: NewCreateVault(func() {}, func(string, string, model.VaultAppearance) error { return nil })},
		{title: "Vault appearance", view: newVaultAppearanceView("Personal", model.DefaultVaultAppearance(), nil, func() {})},
		{title: "Restore from backup", view: auditPage("Restore from backup", func() {}, container.NewVScroll(container.NewVBox()))},
		{title: "Password Audit", view: auditPage("Password Audit", func() {}, container.NewVScroll(container.NewVBox()))},
		{title: "Settings", view: NewSettingsView(nil, nil, window).View},
	}
	var expectedBack, expectedTitle fyne.Position
	for index, page := range views {
		window.SetContent(page.view)
		var backPosition, titlePosition fyne.Position
		var foundBack, foundTitle bool
		for _, object := range test.LaidOutObjects(page.view) {
			switch object := object.(type) {
			case *widget.Button:
				if object.Icon == icons.Back {
					backPosition = fyne.CurrentApp().Driver().AbsolutePositionForObject(object)
					foundBack = true
				}
			case *iconButton:
				if object.Icon == icons.Back {
					backPosition = fyne.CurrentApp().Driver().AbsolutePositionForObject(object)
					foundBack = true
				}
			case *widget.Label:
				if object.Text == page.title {
					titlePosition = fyne.CurrentApp().Driver().AbsolutePositionForObject(object)
					foundTitle = true
				}
			}
		}
		require.True(t, foundBack, "%s back button should be in the shared header", page.title)
		require.True(t, foundTitle, "%s title should be in the shared header", page.title)
		if index == 0 {
			expectedBack, expectedTitle = backPosition, titlePosition
			continue
		}
		require.Equal(t, expectedBack, backPosition, "%s back position", page.title)
		require.Equal(t, expectedTitle, titlePosition, "%s title position", page.title)
	}
}

func TestDefaultVaultAppearanceUsesPawIconAndThemeSurface(t *testing.T) {
	app := test.NewTempApp(t)
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
		view := newVaultIcon(model.DefaultVaultAppearance(), vaultIconLarge)
		renderer := test.WidgetRenderer(view).(*vaultIconRenderer)
		renderer.Layout(view.MinSize())
		palette := vaultIconPalette("none", variant, view.Theme())
		require.Equal(t, palette.background, renderer.background.FillColor)
		require.Equal(t, fyne.NewSquareSize(80), renderer.MinSize())
		require.Equal(t, fyne.NewSquareSize(44), renderer.image.Size())
		require.Equal(t, float32(16), renderer.background.CornerRadius)
		wantResource := icons.PawLight
		if variant == theme.VariantDark {
			wantResource = icons.PawDark
		}
		require.Equal(t, wantResource.Name(), renderer.image.Resource.Name())
	}
}

func TestVaultIconVariantsUsePastelThemePalette(t *testing.T) {
	app := test.NewTempApp(t)
	variants := []struct {
		size      vaultIconSize
		container float32
		glyph     float32
	}{
		{size: vaultIconLarge, container: 80, glyph: 44},
		{size: vaultIconMedium, container: 40, glyph: 24},
		{size: vaultIconSmall, container: 28, glyph: 18},
	}
	for _, themeVariant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: themeVariant})
		for _, sizes := range variants {
			icon := newVaultIcon(model.VaultAppearance{Icon: "briefcase-business", Color: "blue"}, sizes.size)
			renderer := test.WidgetRenderer(icon).(*vaultIconRenderer)
			renderer.Layout(icon.MinSize())
			palette := vaultIconPalette("blue", themeVariant, icon.Theme())
			require.Equal(t, fyne.NewSquareSize(sizes.container), renderer.MinSize())
			require.Equal(t, fyne.NewSquareSize(sizes.glyph), renderer.image.Size())
			require.Equal(t, sizes.container*0.2, renderer.background.CornerRadius)
			require.Equal(t, palette.background, renderer.background.FillColor)
			require.Equal(t, vaultIconResource("briefcase-business").Name(), renderer.image.Resource.Name())
		}
	}
}

func hasVaultAppearance(view fyne.CanvasObject, want model.VaultAppearance) bool {
	for _, object := range test.LaidOutObjects(view) {
		if icon, ok := object.(*vaultIcon); ok && icon.appearance == want {
			return true
		}
	}
	return false
}

func TestVaultSelectorGolden(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			assertViewGolden(t, "vault-selector-"+variantName(variant)+".png", variant, fyne.NewSize(440, 650), selectorPreview)
		})
	}
}

func TestVaultSelectorInteractions(t *testing.T) {
	test.NewTempApp(t)
	opened := ""
	window := test.NewTempWindow(t, NewSelectVault([]VaultChoice{
		{Name: "Personal", Open: func() { opened = "Personal" }},
		{Name: "Work", Open: func() { opened = "Work" }},
		{Name: "Archive", Unlocked: true, Open: func() { opened = "Archive" }},
	}, func() {}))
	window.Resize(fyne.NewSize(440, 650))
	for _, object := range test.LaidOutObjects(window.Content()) {
		if list, ok := object.(*widget.List); ok {
			list.Select(1)
			require.Equal(t, "Work", opened)
			list.Select(2)
			require.Equal(t, "Archive", opened)
			return
		}
	}
	t.Fatal("vault list was not rendered")
}

func selectorPreview() fyne.CanvasObject {
	return NewSelectVault([]VaultChoice{
		{Name: "Personal", Open: func() {}},
		{Name: "Work", Open: func() {}},
		{Name: "Archive", Unlocked: true, Open: func() {}},
	}, func() {})
}

func assertViewGolden(t *testing.T, golden string, variant fyne.ThemeVariant, size fyne.Size, view func() fyne.CanvasObject) {
	t.Helper()
	app := test.NewTempApp(t)
	app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
	window := test.NewTempWindow(t, view())
	window.Resize(size)
	test.AssertImageMatches(t, golden, window.Canvas().Capture())
}

func variantName(variant fyne.ThemeVariant) string {
	if variant == theme.VariantDark {
		return "dark"
	}
	return "light"
}
