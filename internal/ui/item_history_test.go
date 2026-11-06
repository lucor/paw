// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	json "encoding/json/v2"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
)

func TestEntryDetailHistoryActionUsesExistingMetadataFooter(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item := detailFixture(t)
	view, _, err := newEntryDetailWithHistory(context.Background(), item, window, func() {}, func() {}, func() {}, func() {})
	require.NoError(t, err)
	require.Nil(t, detailButton(view, "History"))

	addHistorySnapshotForUI(t, item)
	view, _, err = newEntryDetailWithHistory(context.Background(), item, window, func() {}, func() {}, func() {}, func() {})
	require.NoError(t, err)
	window.SetContent(view)
	historyButton := detailButton(view, "History")
	require.NotNil(t, historyButton)
	for _, width := range []float32{320, 800} {
		window.Resize(fyne.NewSize(width, 640))
		require.Greater(t, historyButton.Size().Width, float32(0))
		require.Greater(t, historyButton.Size().Height, float32(0))
	}
	footerText := strings.Join(auditTexts(view), "\n")
	require.Contains(t, footerText, "Created · "+formatDetailTimestamp(*item.CreationAt))
	require.Contains(t, footerText, "Modified · "+formatDetailTimestamp(*item.ModifiedAt))

	foundFooter := false
	for _, object := range test.LaidOutObjects(view) {
		parent, ok := object.(*fyne.Container)
		if !ok || !containsDirect(parent.Objects, historyButton) {
			continue
		}
		for _, child := range parent.Objects {
			if label, ok := child.(*widget.Label); ok && strings.HasPrefix(label.Text, "Created · ") {
				foundFooter = true
			}
		}
	}
	require.True(t, foundFooter, "History action shares the Created/Modified footer")
}

func TestItemHistoryListDisplaysNewestFirstAndOpensSelectedEntry(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	item := detailFixture(t)
	history := addHistorySnapshotForUI(t, item)
	history.Entries[0].Item.Title = "Oldest"
	history.Entries[1].Item.Title = "Newest"
	var selectedIndex int
	var selected model.HistoryEntry
	backed := false
	view := newItemHistoryView(history, func() { backed = true }, func(index int, entry model.HistoryEntry) {
		selectedIndex, selected = index, entry
	})
	window.SetContent(view)
	window.Resize(fyne.NewSize(380, 640))
	list := historyList(t, view)
	require.Equal(t, 2, list.Length())
	text := strings.Join(auditTexts(view), "\n")
	require.Contains(t, text, formatDetailTimestamp(history.Entries[1].CapturedAt))
	require.NotContains(t, text, "synthetic-secret")
	list.Select(0)
	require.Equal(t, 1, selectedIndex)
	require.Equal(t, "Newest", selected.Item.Title)
	list.Select(1)
	require.Equal(t, 0, selectedIndex)
	require.Equal(t, "Oldest", selected.Item.Title)
	test.Tap(detailIconButton(view, "Back"))
	require.True(t, backed)
}

func TestHistoricalItemDetailReusesSupportedItemPresentation(t *testing.T) {
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	for _, kind := range []string{"login", "password", "note", "ssh"} {
		t.Run(kind, func(t *testing.T) {
			item := detailItemFixture(t, kind)
			snapshot, err := item.Snapshot()
			require.NoError(t, err)
			data, err := json.Marshal(snapshot, json.Deterministic(true))
			require.NoError(t, err)
			var historical model.Item
			require.NoError(t, json.Unmarshal(data, &historical))
			capturedAt := uint64(1705055040)
			view, clear, err := newHistoricalEntryDetailWithClear(context.Background(), &historical, window, capturedAt, func() {}, func() {})
			require.NoError(t, err)
			t.Cleanup(clear)
			window.SetContent(view)
			texts := auditTexts(view)
			historyContext := "History · " + formatDetailTimestamp(capturedAt)
			require.Contains(t, strings.Join(texts, "\n"), historyContext)
			require.Contains(t, texts, item.Title)
			require.NotContains(t, texts, "Edit entry")
			require.NotContains(t, texts, "Delete entry")
			require.NotContains(t, texts, "Save")
			require.Nil(t, detailIconButton(view, "Entry actions"))
			require.Nil(t, detailButton(view, "History"))
			restoreButton := detailButton(view, "Restore")
			require.NotNil(t, restoreButton)
			restoreHeight := restoreButton.Size().Height
			require.Greater(t, restoreHeight, float32(0))
			require.Less(t, restoreHeight, restoreButton.MinSize().Height*2)
			for _, width := range []float32{320, 800} {
				window.Resize(fyne.NewSize(width, 640))
				require.InDelta(t, restoreHeight, restoreButton.Size().Height, 1)
				for _, object := range test.LaidOutObjects(view) {
					if label, ok := object.(*widget.Label); ok && label.Text == historyContext {
						require.Equal(t, fyne.TextAlignTrailing, label.Alignment)
						require.Equal(t, fyne.TextWrapWord, label.Wrapping)
						require.LessOrEqual(t, label.Size().Width, window.Canvas().Size().Width)
					}
				}
			}
			var restoreColumn fyne.CanvasObject
			for _, object := range test.LaidOutObjects(view) {
				parent, ok := object.(*fyne.Container)
				if ok && containsDirect(parent.Objects, restoreButton) {
					restoreColumn = parent
					break
				}
			}
			require.NotNil(t, restoreColumn)
			require.True(t, canvasContainsText(restoreColumn, item.Title), "Restore shares the item title header")
			require.False(t, canvasContainsText(restoreColumn, "Created · "), "Restore belongs in the header, not the timestamp footer")
			require.False(t, canvasContainsText(restoreColumn, historyContext), "history context is in its own row below the header")
			if kind == "login" {
				require.NotContains(t, texts, "synthetic-secret")
				require.NotNil(t, detailIconButton(view, "Show Password"))
			} else if kind == "password" {
				require.NotContains(t, texts, "synthetic-secret")
				require.NotNil(t, detailIconButton(view, "Show Password"))
			} else if kind == "ssh" {
				require.NotContains(t, strings.Join(texts, "\n"), "BEGIN OPENSSH PRIVATE KEY")
				require.NotNil(t, detailIconButton(view, "Show private key"))
			}
			test.Tap(detailIconButton(view, "Back"))
		})
	}
}

func TestItemHistoryNavigationBackCancelAndRestore(t *testing.T) {
	a, window, store, item := newHistoryAppFixture(t)
	a.showItemView(item)

	test.Tap(detailButton(window.Content(), "History"))
	historyScreen := window.Content()
	list := historyList(t, historyScreen)
	list.Select(0)
	historicalScreen := window.Content()
	require.Contains(t, auditTexts(historicalScreen), "History · "+formatDetailTimestamp(200))

	test.Tap(detailIconButton(historicalScreen, "Back"))
	require.NotNil(t, historyList(t, window.Content()))
	test.Tap(detailIconButton(window.Content(), "Back"))
	require.Contains(t, auditTexts(window.Content()), "Current")
	require.NotNil(t, detailIconButton(window.Content(), "Entry actions"))

	// Return to History and open the newest displayed snapshot again.
	test.Tap(detailButton(window.Content(), "History"))
	historyScreen = window.Content()
	historyList(t, historyScreen).Select(0)
	historicalScreen = window.Content()
	test.Tap(detailButton(historicalScreen, "Restore"))
	confirmation := window.Canvas().Overlays().Top()
	require.NotNil(t, confirmation)
	confirmationText := strings.Join(auditTexts(confirmation), "\n")
	require.Contains(t, confirmationText, "Restore previous version?")
	require.Contains(t, confirmationText, "The current version will be saved in History")
	test.Tap(detailButton(confirmation, "Cancel"))
	require.Same(t, historicalScreen, window.Content())
	current, err := store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", current.Title)

	test.Tap(detailButton(historicalScreen, "Restore"))
	confirmation = window.Canvas().Overlays().Top()
	test.Tap(detailButton(confirmation, "Restore"))
	require.Nil(t, window.Canvas().Overlays().Top())
	require.NotContains(t, auditTexts(window.Content()), "History · "+formatDetailTimestamp(200))
	require.Contains(t, auditTexts(window.Content()), "Newest")
	require.NotNil(t, detailButton(window.Content(), "History"))
	current, err = store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Newest", current.Title)
	history, err := current.History()
	require.NoError(t, err)
	require.Len(t, history.Entries, 3)
	require.Equal(t, "Current", history.Entries[2].Item.Title)
}

func TestItemHistoryRestoreRejectsStaleSelection(t *testing.T) {
	a, window, store, item := newHistoryAppFixture(t)
	a.showItemView(item)
	test.Tap(detailButton(window.Content(), "History"))
	historyList(t, window.Content()).Select(0)
	historicalScreen := window.Content()

	current, err := store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	history, err := current.History()
	require.NoError(t, err)
	history.Entries[1].Item.Title = "Changed after opening history"
	require.NoError(t, current.SetHistory(history))
	require.NoError(t, store.StoreItem(a.vault, current))

	test.Tap(detailButton(historicalScreen, "Restore"))
	test.Tap(detailButton(window.Canvas().Overlays().Top(), "Restore"))
	require.Same(t, historicalScreen, window.Content())
	require.NotNil(t, window.Canvas().Overlays().Top(), "stale selection should show an error")
	current, err = store.LoadItem(a.vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, "Current", current.Title)
}

func newHistoryAppFixture(t *testing.T) (*app, fyne.Window, *storage.FileStorage, *model.Item) {
	t.Helper()
	test.NewTempApp(t)
	window := test.NewTempWindow(t, nil)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault, err := store.CreateVault("Personal", "synthetic-master-password")
	require.NoError(t, err)
	item := detailFixture(t)
	history := addHistorySnapshotForUI(t, item)
	history.Entries[0].Item.Title = "Oldest"
	history.Entries[1].Item.Title = "Newest"
	item.Title = "Current"
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	a := &app{
		win:           window,
		main:          container.NewVScroll(widget.NewLabel("Vault")),
		state:         &model.AppState{Settings: model.NewDefaultSettings()},
		storage:       store,
		unlockedVault: map[string]*model.Vault{vault.Accounts[0].ID: vault},
		vault:         vault,
		vaultID:       vault.Accounts[0].ID,
		filter:        map[string]*vaultFilterOptions{},
	}
	t.Cleanup(func() {
		a.closeItemDetail()
		a.closeCodesView()
	})
	return a, window, store, item
}

func addHistorySnapshotForUI(t *testing.T, item *model.Item) model.History {
	t.Helper()
	history, err := item.History()
	require.NoError(t, err)
	oldest := *item
	oldest.Title = "Oldest"
	require.NoError(t, history.AddSnapshot(&oldest, 100))
	newest := *item
	newest.Title = "Newest"
	require.NoError(t, history.AddSnapshot(&newest, 200))
	require.NoError(t, item.SetHistory(history))
	return history
}

func historyList(t *testing.T, view fyne.CanvasObject) *widget.List {
	t.Helper()
	for _, object := range test.LaidOutObjects(view) {
		if list, ok := object.(*widget.List); ok {
			return list
		}
	}
	t.Fatal("history list not found")
	return nil
}

func detailButton(view fyne.CanvasObject, text string) *widget.Button {
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*widget.Button); ok && button.Text == text {
			return button
		}
	}
	return nil
}

func detailIconButton(view fyne.CanvasObject, label string) *iconButton {
	for _, object := range test.LaidOutObjects(view) {
		if button, ok := object.(*iconButton); ok && button.AccessibilityLabel() == label {
			return button
		}
	}
	return nil
}

func containsDirect(objects []fyne.CanvasObject, target fyne.CanvasObject) bool {
	for _, object := range objects {
		if object == target {
			return true
		}
	}
	return false
}

func canvasContainsText(object fyne.CanvasObject, value string) bool {
	if label, ok := object.(*widget.Label); ok {
		return strings.Contains(label.Text, value)
	}
	if parent, ok := object.(*fyne.Container); ok {
		for _, child := range parent.Objects {
			if canvasContainsText(child, value) {
				return true
			}
		}
	}
	return false
}
