// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"github.com/stretchr/testify/require"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

func TestSyncFailedViewShowsPartialResult(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	plan := &pawsync.Plan{Entries: []pawsync.PlanEntry{
		{ItemID: "one", Decision: pawsync.CopyLocalToRemote},
		{ItemID: "two", Decision: pawsync.CopyRemoteToLocal},
		{ItemID: "three", Decision: pawsync.CopyLocalToRemote},
		{ItemID: "four", Decision: pawsync.CopyLocalToRemote},
		{ItemID: "five", Decision: pawsync.CopyLocalToRemote},
	}}
	var labels = make(map[string]bool)
	var statusIcons = make(map[fyne.Resource]bool)
	var done bool
	var button *widget.Button
	fyne.DoAndWait(func() {
		view := newSyncFailedView("fedora", plan, pawsync.ExecutionResult{
			Completed:   plan.Entries[:3],
			Failed:      &plan.Entries[3],
			Unattempted: plan.Entries[4:],
		}, func() { done = true })
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if icon, ok := object.(*widget.Icon); ok {
				statusIcons[icon.Resource] = true
			}
			if candidate, ok := object.(*widget.Button); ok && candidate.Text == "Done" {
				button = candidate
			}
		}
		test.Tap(button)
	})
	require.True(t, labels["Sync failed"])
	require.True(t, labels["3 of 5 operations completed."])
	require.True(t, labels["Copied to fedora"])
	require.True(t, labels["Copied to this device"])
	require.True(t, labels["The current operation may have completed on the other device. Run Sync again to verify the state of your vaults."])
	require.True(t, statusIcons[icons.Check])
	require.True(t, statusIcons[icons.Bug])
	require.True(t, statusIcons[icons.Timer])
	require.True(t, done)
}

func TestSyncCancelledViewReportsCancellationWithoutSuccess(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var labels = make(map[string]bool)
	var done bool
	var doneButton *widget.Button
	fyne.DoAndWait(func() {
		view := newSyncCancelledView("fedora", func() { done = true })
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if button, ok := object.(*widget.Button); ok && button.Text == "Done" {
				doneButton = button
			}
		}
		test.Tap(doneButton)
	})
	require.True(t, labels["Sync cancelled"])
	require.True(t, labels["The other device cancelled Sync."])
	require.True(t, labels["Run Sync again to verify the state of your vaults."])
	require.False(t, labels["Sync completed"])
	require.True(t, done)
}

func TestSyncConnectionLostViewPreservesUnknownOutcome(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	var labels = make(map[string]bool)
	var done bool
	var doneButton *widget.Button
	fyne.DoAndWait(func() {
		view := newSyncConnectionLostView("fedora", func() { done = true })
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels[label.Text] = true
			}
			if button, ok := object.(*widget.Button); ok && button.Text == "Done" {
				doneButton = button
			}
		}
		test.Tap(doneButton)
	})
	require.True(t, labels["Connection lost"])
	require.True(t, labels["The sync outcome is unknown."])
	require.True(t, labels["Run Sync again to verify the state of your vaults."])
	require.False(t, labels["Sync completed"])
	require.False(t, labels["The other device cancelled Sync."])
	require.True(t, done)
}

func TestCloneFailedViewIdentifiesPartialClone(t *testing.T) {
	app := test.NewApp()
	t.Cleanup(app.Quit)
	plan := &pawsync.Plan{Entries: []pawsync.PlanEntry{
		{ItemID: "one", Decision: pawsync.CopyRemoteToLocal},
		{ItemID: "two", Decision: pawsync.CopyRemoteToLocal},
	}}
	var labels []string
	fyne.DoAndWait(func() {
		view := newCloneFailedView("fedora", plan, pawsync.ExecutionResult{
			Completed:   plan.Entries[:1],
			Failed:      &plan.Entries[1],
			Unattempted: nil,
		}, false, true, func() {})
		for _, object := range test.LaidOutObjects(view.view) {
			if label, ok := object.(*widget.Label); ok {
				labels = append(labels, label.Text)
			}
		}
	})
	require.Contains(t, labels, "Clone failed")
	require.Contains(t, labels, "Clone")
	require.Contains(t, labels, "The clone from fedora did not complete.")
	require.Contains(t, labels, "The incomplete vault was removed. The temporary pairing was removed.")
}
