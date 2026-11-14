// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/storage"
	"go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/test"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestSyncDoneViewShowsOperationResults(t *testing.T) {
	app := fynetest.NewApp()
	t.Cleanup(app.Quit)
	var done bool
	var view *syncDoneView
	labels := make(map[string]bool)
	buttons := make(map[string]*widget.Button)
	var checkIcon bool
	var filledCircles int
	fyne.DoAndWait(func() {
		view = newSyncDoneView(
			"fedora",
			sync.PlanSummary{
				CopyLocalToRemote: 4,
				CopyRemoteToLocal: 1,
				DeleteLocal:       1,
				DeleteRemote:      1,
				Skipped:           2,
			},
			"",
			func() { done = true },
		)
		for _, object := range fynetest.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if button, ok := object.(*widget.Button); ok {
				buttons[button.Text] = button
			}
			if icon, ok := object.(*widget.Icon); ok && icon.Resource == icons.CircleCheck {
				checkIcon = true
			}
			if _, ok := object.(*canvas.Circle); ok {
				filledCircles++
			}
		}
		fynetest.Tap(buttons["Done"])
	})
	for _, label := range []string{
		"Sync completed",
		"7 operations completed.",
		"Copied to fedora",
		"Copied to this device",
		"Deleted",
		"Skipped",
	} {
		require.True(t, labels[label], "expected label %q", label)
	}
	require.False(t, labels["Deleted from this device"])
	require.False(t, labels["Deleted from fedora"])
	require.True(t, checkIcon)
	require.Zero(t, filledCircles)
	require.True(t, done)
}

func TestSyncDoneViewUsesSingularOperationCount(t *testing.T) {
	app := fynetest.NewApp()
	t.Cleanup(app.Quit)
	labels := make(map[string]bool)
	fyne.DoAndWait(func() {
		view := newSyncDoneView("fedora", sync.PlanSummary{CopyLocalToRemote: 1}, "", func() {})
		for _, object := range fynetest.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
		}
	})
	require.True(t, labels["1 operation completed."])
	require.False(t, labels["1 operations completed."])
}

func TestSyncDoneRefreshesVaultItems(t *testing.T) {
	testApp := fynetest.NewApp()
	t.Cleanup(testApp.Quit)
	window := testApp.NewWindow("Sync")
	t.Cleanup(window.Close)
	store, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	vault := test.CreateVault(t, store, "Personal")
	vaultID := vault.Accounts[0].ID

	connectView := newSyncConnectView(false, nil, nil, nil, nil, nil)
	run := &syncConnectRun{
		ctx:        context.Background(),
		cancel:     func() {},
		done:       make(chan struct{}),
		view:       connectView,
		vault:      vault,
		peer:       sync.Peer{Name: "fedora"},
		reviewView: &syncReviewView{plan: &sync.Plan{}},
	}
	close(run.done)
	a := &app{
		win:           window,
		main:          container.NewVScroll(widget.NewLabel("stale list")),
		storage:       store,
		vault:         vault,
		vaultID:       vaultID,
		state:         &model.AppState{Settings: model.NewDefaultSettings()},
		filter:        make(map[string]*vaultFilterOptions),
		unlockedVault: map[string]*model.Vault{vaultID: vault},
		syncRun:       run,
	}
	a.main.Content = a.makeCurrentVaultView()
	window.SetContent(a.main)
	window.Show()

	item, err := model.NewNote("Copied during Sync", "contents")
	require.NoError(t, err)
	require.NoError(t, store.StoreItem(vault, item))
	a.showSyncDone(run, "")
	var doneButton *widget.Button
	for _, object := range fynetest.LaidOutObjects(window.Content()) {
		if button, ok := object.(*widget.Button); ok && button.Text == "Done" {
			doneButton = button
		}
	}
	require.NotNil(t, doneButton)
	fynetest.Tap(doneButton)

	var itemVisible bool
	require.Eventually(t, func() bool {
		fyne.DoAndWait(func() {
			for _, object := range fynetest.LaidOutObjects(a.main.Content) {
				if label, ok := object.(*widget.Label); ok && label.Text == item.Title {
					itemVisible = true
				}
			}
		})
		return itemVisible
	}, time.Second, 10*time.Millisecond)
}

func TestSyncDoneViewReportsPeerNotificationProblem(t *testing.T) {
	app := fynetest.NewApp()
	t.Cleanup(app.Quit)
	var labels = make(map[string]bool)
	fyne.DoAndWait(func() {
		view := newSyncDoneView(
			"fedora",
			sync.PlanSummary{},
			"Could not notify fedora that Sync completed: connection closed",
			func() {},
		)
		for _, object := range fynetest.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
		}
	})
	require.True(t, labels["No operations were needed."])
	require.True(t, labels["Could not notify fedora that Sync completed: connection closed"])
}

func TestCloneDoneViewShowsReadyVaultAndCounts(t *testing.T) {
	app := fynetest.NewApp()
	t.Cleanup(app.Quit)
	var labels = make(map[string]bool)
	var iconsSeen = make(map[fyne.Resource]bool)
	var opened bool
	var openButton *widget.Button
	var summaryLabel *widget.Label
	fyne.DoAndWait(func() {
		view := newCloneDoneView(sync.VaultReference{
			Name: "Personal", Items: 1, Collections: 3, Attachments: 2,
		}, func() { opened = true })
		for _, object := range fynetest.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
				if label.Text == "1 item · 2 attachments" {
					summaryLabel = label
				}
			}
			if icon, ok := object.(*widget.Icon); ok {
				iconsSeen[icon.Resource] = true
			}
			if button, ok := object.(*widget.Button); ok && button.Text == "Open vault" {
				openButton = button
			}
		}
		fynetest.Tap(openButton)
	})
	require.True(t, labels["Clone completed"])
	require.True(t, labels[`Vault "Personal" is ready on this device.`])
	require.True(t, labels["1 item · 2 attachments"])
	require.NotNil(t, summaryLabel)
	require.Equal(t, fyne.TextAlignCenter, summaryLabel.Alignment)
	require.False(t, labels["3 collections"])
	require.False(t, labels["Paired"])
	require.True(t, iconsSeen[icons.CircleCheck])
	require.False(t, iconsSeen[icons.CheckWhite])
	require.True(t, opened)
}

func TestCloneSourceDoneViewDoesNotShowPairedRow(t *testing.T) {
	app := fynetest.NewApp()
	t.Cleanup(app.Quit)
	var labels = make(map[string]bool)
	var iconSeen bool
	var done bool
	var doneButton *widget.Button
	fyne.DoAndWait(func() {
		view := newCloneSourceDoneView("fedora", func() { done = true })
		for _, object := range fynetest.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if icon, ok := object.(*widget.Icon); ok && icon.Resource == icons.CircleCheck {
				iconSeen = true
			}
			if button, ok := object.(*widget.Button); ok && button.Text == "Done" {
				doneButton = button
			}
		}
		fynetest.Tap(doneButton)
	})
	require.True(t, labels["Clone completed"])
	require.True(t, labels["The vault has been transferred to fedora."])
	require.False(t, labels["Paired"])
	require.True(t, iconSeen)
	require.True(t, done)
}
