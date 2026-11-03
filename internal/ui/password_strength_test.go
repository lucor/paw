// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

func TestPasswordStrengthSegmentsAndTheme(t *testing.T) {
	app := test.NewTempApp(t)
	i := NewPasswordStrengthIndicator()
	require.False(t, i.Visible())
	window := test.NewTempWindow(t, i)
	window.Resize(fyne.NewSize(320, 40))
	for _, variant := range []fyne.ThemeVariant{theme.VariantLight, theme.VariantDark} {
		app.Settings().SetTheme(variantTheme{Theme: theme.DefaultTheme(), variant: variant})
		var segmentSize fyne.Size
		var labelPosition fyne.Position
		for score, label := range []string{"Very weak", "Weak", "Fair", "Strong", "Very strong"} {
			i.SetScore(score)
			i.Refresh()
			require.True(t, i.Visible())
			require.Equal(t, label, i.label.Text)
			if score == 0 {
				segmentSize = i.segments[0].Size()
				labelPosition = i.label.Position()
			} else {
				require.Equal(t, segmentSize, i.segments[0].Size(), "label length must not resize the bar")
				require.Equal(t, labelPosition, i.label.Position(), "label length must not move the text")
			}
			for index, segment := range i.segments {
				require.Greater(t, segment.Size().Width, float32(20), "segments must expand to fill the row")
				require.Equal(t, float32(6), segment.Size().Height)
				if index <= score {
					require.Equal(t, passwordStrengthColor(score, app.Settings().ThemeVariant()), segment.FillColor)
				} else {
					require.Equal(t, i.Theme().Color(theme.ColorNameInputBorder, variant), segment.FillColor)
				}
			}
		}
	}
	i.Clear()
	require.False(t, i.Visible())
	require.Empty(t, i.label.Text)
	i.SetScore(0)
	require.True(t, i.Visible())
}

func TestCreateVaultUpdatesPasswordStrength(t *testing.T) {
	test.NewTempApp(t)
	view := NewCreateVault(func() {}, func(string, string, model.VaultAppearance) error { return nil })
	window := test.NewTempWindow(t, view)
	window.Resize(fyne.NewSize(440, 650))
	// The hidden indicator remains accessible through the form's items.
	var indicator *PasswordStrengthIndicator
	for _, object := range test.LaidOutObjects(view) {
		if form, ok := object.(*widget.Form); ok {
			field := form.Items[1].Widget.(*PasswordField)
			indicator = field.strength
			password := field.Entry
			require.False(t, indicator.Visible())
			password.SetText("password")
			require.True(t, indicator.Visible())
			require.Greater(t, indicator.Size().Width, float32(100))
			for _, segment := range indicator.segments {
				require.Greater(t, segment.Size().Width, float32(20))
			}
			require.Equal(t, "Very weak", indicator.label.Text)
			gap := indicator.Position().Y - (password.Position().Y + password.Size().Height)
			require.InDelta(t, theme.Padding(), gap, 0.1, "feedback should use only native spacing below the entry")
			password.SetText("9g&TQ2#zK7!mR8$wV5@xL3")
			require.Equal(t, "Very strong", indicator.label.Text)
			password.SetText("")
			require.False(t, indicator.Visible())
		}
	}
	require.NotNil(t, indicator)
}
