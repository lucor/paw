// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"testing"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
)

func TestItemEditorsRequireTitleBeforeSave(t *testing.T) {
	test.NewTempApp(t)
	settings := model.NewDefaultSettings()
	settings.FaviconDownloader.Disabled = true
	passwordSettings := model.PasswordSettings{Mode: paw.CustomPassword}
	login, err := model.NewLogin("Existing login", "name@example.com", "secret", passwordSettings)
	require.NoError(t, err)
	password, err := model.NewPassword("Existing password", "secret", passwordSettings)
	require.NoError(t, err)
	note, err := model.NewNote("Existing note", "note")
	require.NoError(t, err)
	ssh, err := model.NewSSHKey("Existing SSH key", sshDetailFixtureKey(t), "", passwordSettings)
	require.NoError(t, err)

	editors := []struct {
		name   string
		typeID model.ItemType
		item   *model.Item
		new    func(*model.Item, bool) (*EditorTitleState, error)
	}{
		{name: "login", typeID: model.LoginItemType, item: login, new: func(item *model.Item, isNew bool) (*EditorTitleState, error) {
			e, err := NewLoginEditor(context.Background(), item, settings.FaviconDownloader, isNew)
			if err != nil {
				return nil, err
			}
			return &EditorTitleState{e.Title, e.SaveButton, e.UpdatedItem}, nil
		}},
		{name: "password", typeID: model.PasswordItemType, item: password, new: func(item *model.Item, isNew bool) (*EditorTitleState, error) {
			e, err := NewPasswordEditor(item, isNew)
			if err != nil {
				return nil, err
			}
			return &EditorTitleState{e.Title, e.SaveButton, e.UpdatedItem}, nil
		}},
		{name: "note", typeID: model.NoteItemType, item: note, new: func(item *model.Item, isNew bool) (*EditorTitleState, error) {
			e, err := NewNoteEditor(item, isNew)
			if err != nil {
				return nil, err
			}
			return &EditorTitleState{e.Title, e.SaveButton, e.UpdatedItem}, nil
		}},
		{name: "ssh", typeID: model.SSHKeyItemType, item: ssh, new: func(item *model.Item, isNew bool) (*EditorTitleState, error) {
			e, err := NewSSHKeyEditor(item, isNew)
			if err != nil {
				return nil, err
			}
			return &EditorTitleState{e.Title, e.SaveButton, e.UpdatedItem}, nil
		}},
	}
	for _, tc := range editors {
		t.Run(tc.name, func(t *testing.T) {
			template, err := newItemTemplate(tc.typeID)
			require.NoError(t, err)
			require.Empty(t, template.Title)
			created, err := tc.new(template, true)
			require.NoError(t, err)
			require.Empty(t, created.title.Text)
			require.True(t, created.save.Disabled())
			_, err = created.saveItem()
			require.EqualError(t, err, "title is required")
			created.title.SetText(" \t ")
			require.True(t, created.save.Disabled())
			created.title.SetText("New item")
			require.False(t, created.save.Disabled())

			edited, err := tc.new(tc.item, false)
			require.NoError(t, err)
			require.Equal(t, tc.item.Title, edited.title.Text)
			require.False(t, edited.save.Disabled())
		})
	}
}

func TestNewItemEditorStartsWithTitleFocused(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Paw")
	t.Cleanup(window.Close)
	item, err := newItemTemplate(model.NoteItemType)
	require.NoError(t, err)
	a := &app{win: window}
	a.showEditItemView(item, true)
	var title *widget.Entry
	for _, object := range test.LaidOutObjects(window.Content()) {
		if entry, ok := object.(*widget.Entry); ok && !entry.MultiLine {
			title = entry
			break
		}
	}
	require.NotNil(t, title)
	require.Equal(t, title, window.Canvas().Focused())
}

type EditorTitleState struct {
	title    *widget.Entry
	save     *widget.Button
	saveItem func() (*model.Item, error)
}
