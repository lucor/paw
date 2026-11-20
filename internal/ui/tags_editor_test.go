// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/ui/assets/icons"
	"go.lucor.dev/paw/internal/ui/widgets"
)

func TestTagsEditorAddsSuggestedTagsAndRemovesTags(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor := newTagsEditor(item, []string{"Personal", "Work"})
	window := fynetest.NewTempWindow(t, editor.View)
	window.Resize(fyne.NewSize(320, 400))
	require.Equal(t, "Add tag", editor.addButton.Text)
	require.Equal(t, icons.TagPlus.Name(), editor.addButton.Icon.Name())

	fynetest.Tap(findButtonByText(t, editor.View, "Add tag"))
	editor.input.SetText("Work")
	require.Equal(t, []string{"Work"}, editor.tags)
	require.Empty(t, editor.input.Text)
	require.True(t, editor.IsDirty())

	chip := findTagBadge(editor.View)
	require.NotNil(t, chip)
	require.Equal(t, "×", chip.Action.Text)
	fynetest.Tap(chip.Action)
	require.Empty(t, editor.tags)
	require.False(t, editor.IsDirty())
}

func TestTagsEditorAddsNewTagOnSubmitAndCancels(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor := newTagsEditor(item, nil)
	window := fynetest.NewTempWindow(t, editor.View)

	fynetest.Tap(findButtonByText(t, editor.View, "Add tag"))
	require.Same(t, editor.input, window.Canvas().Focused())
	editor.input.SetText("Personal")
	editor.input.OnSubmitted(editor.input.Text)
	require.Equal(t, []string{"Personal"}, editor.tags)
	require.False(t, editor.inputRow.Visible())

	fynetest.Tap(findButtonByText(t, editor.View, "Add tag"))
	editor.input.SetText("Temporary")
	editor.cancelAdd()
	require.Equal(t, []string{"Personal"}, editor.tags)
	require.False(t, editor.inputRow.Visible())
}

func findTagBadge(object fyne.CanvasObject) *widgets.Badge {
	if badge, ok := object.(*widgets.Badge); ok {
		return badge
	}
	if group, ok := object.(*fyne.Container); ok {
		for _, child := range group.Objects {
			if badge := findTagBadge(child); badge != nil {
				return badge
			}
		}
	}
	return nil
}

func TestTagChipsWrapInNarrowEditor(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	item.Tags = []string{"Personal", "Recovery", "Travel"}
	editor := newTagsEditor(item, nil)
	fynetest.NewTempWindow(t, editor.View)
	editor.View.Resize(fyne.NewSize(180, 140))
	wrap := editor.rows.Objects[0].(*fyne.Container)
	wrap.Resize(fyne.NewSize(180, 120))
	first := wrap.Objects[0].Position()
	second := wrap.Objects[1].Position()
	require.NotEqual(t, first.Y, second.Y, "chips should continue on another row when they do not fit")
}

func TestItemEditorsSaveTagsForEveryCategory(t *testing.T) {
	fynetest.NewTempApp(t)
	login := newTestLoginEditor(t, loginEditorFixture(t), false)
	passwordItem, err := model.NewPassword("Password", "secret", model.PasswordSettings{})
	require.NoError(t, err)
	password, err := NewPasswordEditor(passwordItem, false)
	require.NoError(t, err)
	note, err := NewNoteEditor(noteEditorFixture(t), false)
	require.NoError(t, err)
	sshItem, _ := sshDetailFixture(t)
	ssh, err := NewSSHKeyEditor(sshItem, false)
	require.NoError(t, err)

	editors := []struct {
		name string
		item *model.Item
		tags *tagsEditor
		save func() (*model.Item, error)
	}{
		{name: "login", item: login.item, tags: login.tags, save: login.UpdatedItem},
		{name: "password", item: password.item, tags: password.tags, save: password.UpdatedItem},
		{name: "note", item: note.item, tags: note.tags, save: note.UpdatedItem},
		{name: "SSH key", item: ssh.item, tags: ssh.tags, save: ssh.UpdatedItem},
	}
	for _, editor := range editors {
		editor := editor
		t.Run(editor.name, func(t *testing.T) {
			editor.tags.input.SetText("team")
			editor.tags.addTag()
			require.True(t, editor.tags.IsDirty())
			updated, err := editor.save()
			require.NoError(t, err)
			require.Equal(t, []string{"team"}, updated.Tags)
			require.Empty(t, editor.item.Tags, "editing must not mutate the original item before save")
		})
	}
}

func findButtonByText(t *testing.T, object fyne.CanvasObject, text string) *widget.Button {
	t.Helper()
	if button, ok := object.(*widget.Button); ok && button.Text == text {
		return button
	}
	if group, ok := object.(*fyne.Container); ok {
		for _, child := range group.Objects {
			if button := findButtonByText(t, child, text); button != nil {
				return button
			}
		}
	}
	return nil
}
