// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"errors"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestCloneCreateViewRequiresMatchingLocalPassword(t *testing.T) {
	testApp := test.NewApp()
	t.Cleanup(testApp.Quit)
	var name, password, defaultName string
	var labels []string
	var view *cloneCreateView
	fyne.DoAndWait(func() {
		view = newCloneCreateView("phoenix.localdomain", pawsync.VaultReference{
			Name:  "Work vault",
			Items: 5, Collections: 3, Attachments: 1,
		}, func() {}, func(gotName, gotPassword string) error {
			name = gotName
			password = gotPassword
			return nil
		})
		defaultName = view.name.Text
		view.password.Entry.SetText("local secret")
		require.True(t, view.password.strength.Visible())
		view.confirm.SetText("different secret")
		view.confirm.Validate()
		require.True(t, view.create.Disabled())
		view.confirm.SetText("local secret")
		require.False(t, view.create.Disabled())
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels = append(labels, label.Text)
			}
		}
		test.Tap(view.create)
	})
	require.Contains(t, labels, "Review vault")
	require.Contains(t, labels, "Clone from phoenix")
	require.Contains(t, labels, "5 items · 1 attachment")
	require.NotContains(t, labels, "Work vault")
	require.NotContains(t, labels, "Source vault")
	require.NotContains(t, labels, "Device")
	require.NotContains(t, labels, "New vault")
	require.NotContains(t, labels, "3 collections")
	require.Equal(t, "Work vault", defaultName)
	require.Equal(t, "Work vault", name)
	require.Equal(t, "local secret", password)
	var singularAndPluralCounts bool
	fyne.DoAndWait(func() {
		view := newCloneCreateView("phoenix", pawsync.VaultReference{Items: 1, Attachments: 3}, func() {}, nil)
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok && label.Text == "1 item · 3 attachments" {
				singularAndPluralCounts = true
			}
		}
	})
	require.True(t, singularAndPluralCounts)
}

func TestCloneCreateViewFitsNarrowWindowWithLongDeviceName(t *testing.T) {
	app := test.NewTempApp(t)
	window := app.NewWindow("Review vault")
	t.Cleanup(window.Close)
	view := newCloneCreateView(strings.Repeat("long-device-name", 8)+".localdomain", pawsync.VaultReference{
		Name: "Work vault", Items: 5, Attachments: 1,
	}, func() {}, func(string, string) error { return nil })
	window.SetContent(view.view)
	window.Resize(fyne.NewSize(320, 540))
	fyne.DoAndWait(func() {
		view.password.Entry.SetText("weak password")
		view.confirm.SetText("different password")
		require.Error(t, view.confirm.Validate())
	})
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
		fyne.DoAndWait(func() {
			var summary *widget.Label
			for _, object := range test.LaidOutObjects(view.view) {
				if label, ok := object.(*widget.Label); ok && strings.HasPrefix(label.Text, "Clone from ") {
					summary = label
				}
			}
			require.Equal(t, container.ScrollVerticalOnly, view.scroll.Direction)
			require.LessOrEqual(t, view.scroll.Content.Size().Width, view.scroll.Size().Width)
			require.NotNil(t, summary)
			require.Equal(t, fyne.TextWrapBreak, summary.Wrapping)
			require.NotContains(t, summary.Text, ".localdomain")
			require.Greater(t, view.scroll.Content.Size().Height, view.scroll.Size().Height)
			buttonPosition := app.Driver().AbsolutePositionForObject(view.create)
			canvasHeight := window.Canvas().Size().Height
			require.GreaterOrEqual(t, buttonPosition.Y, float32(0))
			require.LessOrEqual(t, buttonPosition.Y+view.create.Size().Height, canvasHeight)
		})
	}
}

func TestCloneCreateViewShowsCreationError(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var message string
	fyne.DoAndWait(func() {
		view := newCloneCreateView("phoenix", pawsync.VaultReference{Name: "Work vault"}, func() {}, func(string, string) error {
			return errors.New("storage failed")
		})
		view.password.Entry.SetText("local secret")
		view.confirm.SetText("local secret")
		test.Tap(view.create)
		message = view.error.Text
	})
	require.Equal(t, "storage failed", message)
}
