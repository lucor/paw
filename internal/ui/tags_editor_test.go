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
)

func TestTagsEditorAddsSuggestedTagsAndRemovesTags(t *testing.T) {
	fynetest.NewTempApp(t)
	item, err := model.NewNote("Example", "note")
	require.NoError(t, err)
	editor := newTagsEditor(item, []string{"Personal", "Work"})
	window := fynetest.NewTempWindow(t, editor.View)
	window.Resize(fyne.NewSize(320, 400))

	editor.input.SetText("Work")
	fynetest.Tap(findButtonByText(t, editor.View, "Add"))
	require.Equal(t, []string{"Work"}, editor.tags)
	require.Empty(t, editor.input.Text)
	require.True(t, editor.IsDirty())

	remove := findDeleteAction(editor.View)
	require.NotNil(t, remove)
	fynetest.Tap(remove)
	require.Empty(t, editor.tags)
	require.False(t, editor.IsDirty())
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
			editor.tags.add()
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
