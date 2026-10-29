// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
)

// TestCreateVaultValidatesAndSubmitsFields checks required input and password confirmation.
func TestCreateVaultValidatesAndSubmitsFields(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Create vault")
	t.Cleanup(window.Close)
	var submittedName, submittedPassword string
	view := NewCreateVault(func() {}, func(name, password string) error {
		submittedName, submittedPassword = name, password
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
	entries[0].SetText("Personal")
	entries[1].SetText("master password")
	entries[2].SetText("different password")
	require.True(t, create.Disabled(), "mismatched passwords must disable submission")
	require.Error(t, form.Validate())
	entries[2].SetText("master password")
	require.NoError(t, form.Validate())
	require.False(t, create.Disabled())
	test.Tap(create)
	require.Equal(t, "Personal", submittedName)
	require.Equal(t, "master password", submittedPassword)
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Create vault" {
			require.Equal(t, widget.HighImportance, button.Importance)
			return
		}
	}
	t.Fatal("Create vault button was not rendered")
}

// TestCreateVaultKeepsActionVisible checks the form remains scrollable in a short window.
func TestCreateVaultKeepsActionVisible(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	window := app.NewWindow("Create vault")
	t.Cleanup(window.Close)
	view := NewCreateVault(func() {}, func(string, string) error { return nil })
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
