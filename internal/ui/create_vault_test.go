// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestCreateVaultAppearanceSelectionReturnsToPreview(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	var submitted storage.VaultAppearance
	view := NewCreateVault(func() {}, func(name, password string, appearance storage.VaultAppearance) error {
		submitted = appearance
		return nil
	})
	window.SetContent(view)
	window.Resize(fyne.NewSize(320, 540))
	var previewButton *widget.Button
	var nameEntry *widget.Entry
	for _, object := range test.LaidOutObjects(view) {
		if entry, ok := object.(*widget.Entry); ok && entry.PlaceHolder == "e.g. Personal" {
			nameEntry = entry
		}
		if button, ok := object.(*widget.Button); ok && button.Size().Width == 80 && button.Icon == nil {
			previewButton = button
		}
	}
	require.NotNil(t, nameEntry)
	nameEntry.SetText("Work")
	require.NotNil(t, previewButton, "the full preview must be a tappable button")
	previewPosition := fyne.CurrentApp().Driver().AbsolutePositionForObject(previewButton)
	test.TapCanvas(window.Canvas(), previewPosition.Add(fyne.NewPos(8, 8)))
	require.Contains(t, auditTexts(window.Content()), "Vault appearance")

	var iconChoice *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Icon != nil && strings.Contains(button.Icon.Name(), "briefcase-business") {
			iconChoice = button
			break
		}
	}
	require.NotNil(t, iconChoice)
	test.Tap(iconChoice)

	var colorSwatch *canvas.Circle
	for _, object := range test.LaidOutObjects(window.Content()) {
		if circle, ok := object.(*canvas.Circle); ok && circle.FillColor == selectedVaultColor("green") {
			colorSwatch = circle
			break
		}
	}
	require.NotNil(t, colorSwatch)
	position := fyne.CurrentApp().Driver().AbsolutePositionForObject(colorSwatch).Add(fyne.NewPos(colorSwatch.Size().Width/2, colorSwatch.Size().Height/2))
	test.TapCanvas(window.Canvas(), position)
	require.Contains(t, auditTexts(window.Content()), "Work")

	var backButton *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Icon == icons.Back {
			backButton = button
			break
		}
	}
	require.NotNil(t, backButton)
	test.Tap(backButton)

	var hasGreen bool
	for _, object := range test.LaidOutObjects(window.Content()) {
		if icon, ok := object.(*vaultIcon); ok && icon.appearance.Color == "green" {
			hasGreen = true
		}
	}
	require.True(t, hasGreen, "the returned preview should show the selected color")

	var entries []*widget.Entry
	var form *widget.Form
	var create *widget.Button
	for _, object := range test.LaidOutObjects(window.Content()) {
		switch object := object.(type) {
		case *widget.Entry:
			entries = append(entries, object)
		case *widget.Form:
			form = object
		case *widget.Button:
			if object.Text == "Create vault" {
				create = object
			}
		}
	}
	require.Len(t, entries, 3)
	require.NotNil(t, form)
	entries[0].SetText("Work")
	entries[1].SetText("test password")
	entries[2].SetText("test password")
	test.Tap(create)
	require.Equal(t, storage.VaultAppearance{Icon: "briefcase-business", Color: "green"}, submitted)
}

// TestCreateVaultValidatesAndSubmitsFields checks required input and password confirmation.
func TestCreateVaultValidatesAndSubmitsFields(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Create vault")
	t.Cleanup(window.Close)
	var submittedName, submittedPassword string
	var submittedAppearance storage.VaultAppearance
	view := NewCreateVault(func() {}, func(name, password string, appearance storage.VaultAppearance) error {
		submittedName, submittedPassword, submittedAppearance = name, password, appearance
		return nil
	})
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 650))
	window.Show()

	var entries []*widget.Entry
	var form *widget.Form
	var create *widget.Button
	for _, object := range test.LaidOutObjects(view) {
		switch object := object.(type) {
		case *widget.Entry:
			entries = append(entries, object)
		case *widget.Form:
			form = object
		case *widget.Button:
			if object.Text == "Create vault" {
				create = object
			}
		}
	}
	require.Len(t, entries, 3)
	require.Equal(t, "e.g. Personal", entries[0].PlaceHolder)
	require.Empty(t, entries[1].PlaceHolder)
	require.Empty(t, entries[2].PlaceHolder)
	require.NotNil(t, form)
	require.NotNil(t, create)
	require.Len(t, form.Items, 3)
	for _, item := range form.Items {
		require.False(t, item.Required, "native form required markers should be omitted")
	}
	require.True(t, create.Disabled(), "required fields must disable submission while empty")
	require.Greater(t, create.Size().Width, float32(350), "submit button should span the form width")
	entries[0].SetText("  ")
	require.True(t, create.Disabled(), "whitespace-only vault name must disable submission")
	create.OnTapped()
	require.Empty(t, submittedName, "submit handler must reject whitespace-only names")
	entries[0].SetText("Personal")
	entries[1].SetText("master password")
	confirm := entries[2]
	window.Canvas().Focus(confirm)
	confirm.SetText("different password")
	require.ErrorContains(t, confirm.Validate(), "passwords do not match")
	window.Canvas().Focus(nil)
	require.True(t, create.Disabled(), "mismatched passwords must disable submission")
	require.Error(t, form.Validate())
	confirm.SetText("master password")
	require.NoError(t, confirm.Validate(), "matching passwords should clear the error immediately")
	require.NoError(t, form.Validate())
	require.False(t, create.Disabled())
	test.Tap(create)
	require.Equal(t, "Personal", submittedName)
	require.Equal(t, "master password", submittedPassword)
	require.Equal(t, storage.DefaultVaultAppearance(), submittedAppearance)
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Create vault" {
			require.Equal(t, widget.HighImportance, button.Importance)
			return
		}
	}
	t.Fatal("Create vault button was not rendered")
}

func TestCreateVaultUsesCompactCenteredVaultIcon(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Create vault")
	t.Cleanup(window.Close)
	view := NewCreateVault(func() {}, func(string, string, storage.VaultAppearance) error { return nil })
	window.SetContent(view)
	window.Resize(fyne.NewSize(320, 540))
	window.Show()
	var appearanceIcon *vaultIcon
	var previewButton *widget.Button
	var pencil *widget.Icon
	for _, object := range test.LaidOutObjects(view) {
		switch object := object.(type) {
		case *vaultIcon:
			if object.size == vaultIconLarge {
				appearanceIcon = object
			}
		case *widget.Icon:
			if object.Resource == icons.Pencil {
				pencil = object
			}
		case *widget.Button:
			if object.Icon == nil && object.Size().Width == 80 {
				previewButton = object
			}
		}
	}
	require.NotNil(t, appearanceIcon)
	require.NotNil(t, previewButton)
	require.NotNil(t, pencil)
	require.Equal(t, fyne.NewSquareSize(80), appearanceIcon.MinSize())
	renderer := test.WidgetRenderer(appearanceIcon).(*vaultIconRenderer)
	require.NotNil(t, renderer)
	require.Equal(t, fyne.NewSquareSize(44), renderer.image.Size())
	previewPosition := app.Driver().AbsolutePositionForObject(previewButton)
	pencilPosition := app.Driver().AbsolutePositionForObject(pencil)
	require.GreaterOrEqual(t, pencilPosition.X, previewPosition.X+previewButton.Size().Width-20)
	require.GreaterOrEqual(t, pencilPosition.Y, previewPosition.Y+previewButton.Size().Height-20)
	require.LessOrEqual(t, pencilPosition.X+pencil.Size().Width, previewPosition.X+previewButton.Size().Width)
	require.LessOrEqual(t, pencilPosition.Y+pencil.Size().Height, previewPosition.Y+previewButton.Size().Height)
}

// TestCreateVaultKeepsActionVisible checks the form remains scrollable in a short window.
func TestCreateVaultKeepsActionVisible(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Create vault")
	t.Cleanup(window.Close)
	view := NewCreateVault(func() {}, func(string, string, storage.VaultAppearance) error { return nil })
	window.SetContent(view)
	window.Resize(fyne.NewSize(440, 380))
	window.Show()
	var form *widget.Form
	var scroll *container.Scroll
	for _, object := range test.LaidOutObjects(view) {
		if candidate, ok := object.(*widget.Form); ok {
			form = candidate
		}
		if candidate, ok := object.(*container.Scroll); ok {
			scroll = candidate
		}
	}
	require.NotNil(t, form)
	require.NotNil(t, scroll)
	require.Equal(t, widget.Vertical, form.Orientation)
}

func TestCreateVaultNarrowLayoutFitsViewport(t *testing.T) {
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		t.Run(variantName(variant), func(t *testing.T) {
			app := test.NewApp()
			app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
			t.Cleanup(app.Quit)
			window := app.NewWindow("Create vault")
			t.Cleanup(window.Close)
			view := NewCreateVault(func() {}, func(string, string, storage.VaultAppearance) error { return nil })
			window.SetContent(view)
			window.Resize(fyne.NewSize(320, 540))
			window.Show()

			objects := test.LaidOutObjects(view)
			var hints []*widget.Label
			for _, object := range objects {
				switch object := object.(type) {
				case *widget.Entry:
					position := app.Driver().AbsolutePositionForObject(object)
					require.GreaterOrEqual(t, position.X, float32(24))
					require.LessOrEqual(t, position.X+object.Size().Width, float32(296))
				case *widget.Label:
					if strings.Contains(object.Text, "Identify this vault") {
						hints = append(hints, object)
					}
				}
			}
			require.Len(t, hints, 1)
			hint := hints[0]
			hintPosition := app.Driver().AbsolutePositionForObject(hint)
			require.GreaterOrEqual(t, hintPosition.X, float32(24))
			require.LessOrEqual(t, hintPosition.X+hint.Size().Width, float32(296))
			require.Equal(t, fyne.TextWrapWord, hint.Wrapping)
			fontSize := fyne.CurrentApp().Settings().Theme().Size(theme.SizeNameCaptionText)
			require.LessOrEqual(t, fyne.MeasureText(hint.Text, fontSize, hint.TextStyle).Width, hint.Size().Width)
		})
	}
}
