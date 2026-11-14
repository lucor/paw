// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncReceiverProgressViewTracksIncomingItems(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var labels []string
	var canceled bool
	fyne.DoAndWait(func() {
		view := newSyncReceiverProgressView("fedora", func() { canceled = true }, false)
		view.Apply(pawsync.SyncProgressEvent{
			ItemID:  "first",
			Title:   "GitHub",
			Type:    model.LoginItemType,
			Started: true,
		})
		view.Apply(pawsync.SyncProgressEvent{
			ItemID:    "first",
			Title:     "GitHub",
			Type:      model.LoginItemType,
			Completed: true,
		})
		view.Apply(pawsync.SyncProgressEvent{
			ItemID:  "second",
			Title:   "Meeting notes",
			Type:    model.NoteItemType,
			Started: true,
		})
		view.Finish(true)
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels = append(labels, label.Text)
			}
		}
		test.Tap(view.cancel)
	})
	require.Contains(t, labels, "Sync completed")
	require.NotContains(t, labels, "Receiving changes…")
	require.NotContains(t, labels, "Syncing with fedora.")
	require.NotContains(t, labels, "GitHub")
	require.NotContains(t, labels, "In progress")
	require.Contains(t, labels, "1 operation completed.")
	require.Contains(t, labels, "Received from fedora")
	require.Contains(t, labels, "1")
	require.True(t, canceled)

	fyne.DoAndWait(func() {
		view := newSyncReceiverProgressView("fedora", func() {}, false)
		view.Apply(pawsync.SyncProgressEvent{
			ItemID:  "first",
			Title:   "GitHub",
			Type:    model.LoginItemType,
			Started: true,
		})
		view.Apply(pawsync.SyncProgressEvent{
			ItemID: "first",
			Failed: true,
		})
		require.Equal(t, "Failed", view.rows["first"].status.Text)
		view.Finish(false)
		require.Equal(t, "The other device did not complete Sync.", view.count.Text)
		require.Equal(t, "Run Sync again to verify the state of your vaults.", view.status.Text)
	})
}

func TestSyncReceiverProgressEventsUpdateTheActiveWindow(t *testing.T) {
	testApp := test.NewApp()
	t.Cleanup(testApp.Quit)
	window := testApp.NewWindow("Sync")
	t.Cleanup(window.Close)
	ctx, cancel := context.WithCancel(context.Background())
	run := &syncConnectRun{
		incomingSyncPeerName: "fedora",
		ctx:                  ctx,
		cancel:               cancel,
		done:                 make(chan struct{}),
	}
	close(run.done)
	run.view = newSyncConnectView(false, func() {}, func() {}, func() {}, func(pawsync.Peer, bool) {}, func(string) {})
	a := &app{win: window, syncRun: run, main: container.NewScroll(widget.NewLabel("Vault"))}
	event := pawsync.SyncProgressEvent{
		DeviceName: "fedora",
		ItemID:     "first",
		Title:      "GitHub",
		Type:       model.LoginItemType,
		Started:    true,
	}
	a.showSyncReceiverProgress(run, event)
	var visible bool
	var activeStatus string
	fyne.DoAndWait(func() {
		visible = window.Content() == run.receiverProgressView.view
		activeStatus = run.receiverProgressView.rows["first"].status.Text
	})
	require.True(t, visible)
	require.Equal(t, "In progress", activeStatus)

	pending := make(chan pawsync.SyncProgressEvent, 1)
	pending <- pawsync.SyncProgressEvent{
		DeviceName: "fedora",
		ItemID:     "first",
		Completed:  true,
	}
	a.finishSyncReceiverProgress(run, true, pending)
	var labels []string
	var buttonText string
	var hasActivity, hasProgressBar bool
	fyne.DoAndWait(func() {
		buttonText = run.receiverProgressView.cancel.Text
		for _, object := range test.LaidOutObjects(run.receiverProgressView.view) {
			if label, ok := object.(*widget.Label); ok {
				labels = append(labels, label.Text)
			}
			switch object.(type) {
			case *widget.Activity:
				hasActivity = true
			case *widget.ProgressBarInfinite:
				hasProgressBar = true
			}
		}
		test.Tap(run.receiverProgressView.cancel)
	})
	require.Contains(t, labels, "1 operation completed.")
	require.Contains(t, labels, "Received from fedora")
	require.Equal(t, "Done", buttonText)
	require.False(t, hasActivity)
	require.False(t, hasProgressBar)
	require.Nil(t, run.receiverProgressView.activity)
	require.Nil(t, run.receiverProgressView.progress)
	require.Nil(t, run.receiverProgressView.items)
	require.Nil(t, run.receiverProgressView.rows)
	require.Nil(t, run.receiverProgressView.status)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.Nil(t, a.syncRun)
}

func TestSyncReceiverSuccessUsesPluralCountForConfirmedEvents(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var labels []string
	fyne.DoAndWait(func() {
		view := newSyncReceiverProgressView("fedora", func() {}, false)
		for _, id := range []string{"first", "second"} {
			view.Apply(pawsync.SyncProgressEvent{ItemID: id, Completed: true})
		}
		view.Finish(true)
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels = append(labels, label.Text)
			}
		}
	})
	require.Contains(t, labels, "2 operations completed.")
	require.Contains(t, labels, "Received from fedora")
	require.Contains(t, labels, "2")
}

func TestSyncWaitingViewShowsPeerReviewState(t *testing.T) {
	testApp := test.NewApp()
	t.Cleanup(testApp.Quit)
	window := testApp.NewWindow("Sync")
	t.Cleanup(window.Close)
	run := &syncConnectRun{}
	a := &app{win: window, syncRun: run}
	labels := make(map[string]bool)
	var centered bool
	fyne.DoAndWait(func() {
		a.showSyncWaiting(run, "phoenix")
		window.Resize(fyne.NewSize(320, 540))
		for _, object := range test.LaidOutObjects(window.Content()) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
				if label.Text == "The other device is reviewing changes." {
					centered = label.Alignment == fyne.TextAlignCenter && label.Size().Width > 100
				}
			}
		}
	})
	require.True(t, labels["Waiting for sync"])
	require.True(t, labels["Connected to phoenix."])
	require.True(t, labels["The other device is reviewing changes."])
	require.True(t, centered)
}

func TestLateSyncEventsDoNotReplaceCurrentViewAfterNavigation(t *testing.T) {
	testApp := test.NewApp()
	t.Cleanup(testApp.Quit)
	window := testApp.NewWindow("Sync")
	t.Cleanup(window.Close)
	currentView := widget.NewLabel("Vault")
	staleRun := &syncConnectRun{incomingSyncPeerName: "fedora"}
	activeRun := &syncConnectRun{}
	a := &app{win: window, syncRun: activeRun}
	fyne.DoAndWait(func() { window.SetContent(currentView) })

	// A queued waiting notification and a late progress event belong to the old run.
	a.showSyncWaiting(staleRun, "fedora")
	a.showSyncReceiverProgress(staleRun, pawsync.SyncProgressEvent{
		DeviceName: "fedora",
		ItemID:     "late",
		Title:      "Late item",
		Started:    true,
	})

	var gotContent fyne.CanvasObject
	fyne.DoAndWait(func() { gotContent = window.Content() })
	require.Same(t, currentView, gotContent)
	require.Nil(t, staleRun.receiverProgressView)
}
