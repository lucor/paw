// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
)

func TestSyncProgressViewTracksSequentialOperationsAndSkips(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "local",
			Type:        model.LoginItemType,
			Title:       "GitHub",
			ContentHash: reviewHash(1),
			StateHash:   reviewHash(1),
		},
	}}
	remote := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "remote",
			Type:        model.NoteItemType,
			Title:       "Notes",
			ContentHash: reviewHash(2),
			StateHash:   reviewHash(2),
		},
	}}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	require.NoError(t, plan.SetDecision("local", pawsync.CopyLocalToRemote))
	require.NoError(t, plan.SetDecision("remote", pawsync.Skip))
	entries := append([]pawsync.PlanEntry(nil), plan.Entries...)
	var cancelRequested bool
	var view *syncProgressView
	var initialCount, activeStatus, completedCount, skippedStatus, finalStatus string
	var cancelDisabled bool
	fyne.DoAndWait(func() {
		view = newSyncProgressView("fedora", local, remote, plan, func() { cancelRequested = true })
		initialCount = view.count.Text
		view.Apply(entries[0], true, nil)
		activeStatus = view.rows[entries[0].ItemID].status.Text
		view.Apply(entries[0], false, nil)
		completedCount = view.count.Text
		view.Apply(entries[1], true, nil)
		view.Apply(entries[1], false, nil)
		skippedStatus = view.rows[entries[1].ItemID].status.Text
		view.Finish(pawsync.ExecutionResult{Completed: entries}, nil)
		finalStatus = view.status.Text
		cancelDisabled = view.cancel.Disabled()
	})
	require.Equal(t, "0 of 1 operations completed", initialCount)
	require.Equal(t, "In progress", activeStatus)
	require.Equal(t, "1 of 1 operations completed", completedCount)
	require.Equal(t, "Skipped", skippedStatus)
	require.Equal(t, "Sync completed.", finalStatus)
	require.True(t, cancelDisabled)

	var cancelButton *widget.Button
	fyne.DoAndWait(func() {
		view = newSyncProgressView("fedora", local, remote, plan, func() { cancelRequested = true })
		cancelButton = view.cancel
		test.Tap(cancelButton)
	})
	require.True(t, cancelRequested)
}

func TestSyncProgressViewReportsCancellationAndUncertainRemoteOutcome(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "local",
			Type:        model.PasswordItemType,
			Title:       "VPN",
			ContentHash: reviewHash(1),
			StateHash:   reviewHash(1),
		},
	}}
	remote := &pawsync.Manifest{}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	entry := plan.Entries[0]
	var state, status string
	fyne.DoAndWait(func() {
		view := newSyncProgressView("fedora", local, remote, plan, func() {})
		view.Apply(entry, true, nil)
		view.Apply(entry, false, context.Canceled)
		view.Finish(pawsync.ExecutionResult{Failed: &entry}, context.Canceled)
		state = view.rows[entry.ItemID].status.Text
		status = view.status.Text
	})
	require.Equal(t, "Cancelled", state)
	require.Contains(t, status, "Sync cancelled after 0 of 1 operations")
	require.Contains(t, status, "may have completed on the other device")
}

func TestSyncConfirmStartsTransferProgressAndCancellation(t *testing.T) {
	driver := test.NewApp()
	t.Cleanup(driver.Quit)
	window := driver.NewWindow("Sync")
	t.Cleanup(window.Close)
	local := &pawsync.Manifest{Items: []pawsync.ManifestItem{
		{
			ID:          "local",
			Type:        model.LoginItemType,
			Title:       "GitHub",
			ContentHash: reviewHash(1),
			StateHash:   reviewHash(1),
		},
	}}
	remote := &pawsync.Manifest{}
	diff, err := pawsync.BuildDiff(local, remote)
	require.NoError(t, err)
	plan, err := pawsync.BuildPlan(diff, local, remote)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	run := &syncConnectRun{
		ctx:              ctx,
		cancel:           cancel,
		transferRequests: make(chan struct{}, 1),
		peer:             pawsync.Peer{Name: "fedora"},
		reviewView: &syncReviewView{
			plan:       plan,
			local:      local,
			remote:     remote,
			deviceName: "fedora",
		},
	}
	a := &app{
		win:     window,
		syncRun: run,
	}
	var confirmView *syncConfirmView
	var progressVisible bool
	var transferQueued bool
	var transferCanceled bool
	fyne.DoAndWait(func() {
		confirmView = newSyncConfirmView(
			"fedora",
			pawsync.VaultReference{Name: "Personal"},
			pawsync.VaultReference{Name: "Personal"},
			local,
			remote,
			plan,
			func() {},
			func() { a.startSyncTransfer(run, confirmView) },
			func() {},
		)
		window.SetContent(confirmView.view)
		test.Tap(confirmView.sync)
		progressVisible = window.Content() == run.progressView.view
	})
	require.True(t, progressVisible)
	select {
	case <-run.transferRequests:
		transferQueued = true
	default:
	}
	require.True(t, transferQueued)
	fyne.DoAndWait(func() {
		test.Tap(run.progressView.cancel)
		transferCanceled = run.transferCtx.Err() == context.Canceled
	})
	require.True(t, transferCanceled)
}
