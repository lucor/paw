// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type syncProgressView struct {
	view       fyne.CanvasObject
	count      *widget.Label
	status     *widget.Label
	cancel     *widget.Button
	rows       map[string]*syncProgressRow
	completed  int
	total      int
	deviceName string
	clone      bool
	progress   *widget.ProgressBarInfinite
}

type syncProgressRow struct {
	activity *widget.Activity
	status   *widget.Label
	started  bool
}

func newSyncProgressView(
	deviceName string,
	local, remote *pawsync.Manifest,
	plan *pawsync.Plan,
	cancel func(),
	clone ...bool,
) *syncProgressView {
	isClone := len(clone) > 0 && clone[0]
	back := widget.NewButtonWithIcon("", icons.Back, cancel)
	back.Importance = widget.LowImportance
	title := "Sync"
	if isClone {
		title = "Clone"
	}
	header := newPageHeader(back, newText(title, titleStyle), nil)
	view := &syncProgressView{
		rows:       make(map[string]*syncProgressRow),
		deviceName: deviceName,
		total:      syncOperationCount(plan.Summary()),
		count:      newText("0 of 0 operations completed", sectionTitleStyle),
		status:     newText("", captionStyle),
		clone:      isClone,
	}
	if isClone {
		view.progress = widget.NewProgressBarInfinite()
		view.progress.Start()
		view.cancel = widget.NewButton("Cancel", cancel)
		footer := container.NewBorder(widget.NewSeparator(), nil, nil, view.cancel, nil)
		content := container.NewVBox(
			newText("Receiving from "+deviceName, headingStyle),
			newText("Receiving vault data…", bodyStyle),
			view.progress,
		)
		view.view = newSecondaryPage(header, footer, container.NewVScroll(content))
		return view
	}
	content := []fyne.CanvasObject{
		container.NewHBox(navigationRowIcon(icons.Laptop), newText("Syncing with "+deviceName, bodyStyle)),
		view.count,
		view.status,
	}
	localItems := manifestItemsByID(local)
	remoteItems := manifestItemsByID(remote)
	for index, entry := range plan.Entries {
		if index > 0 {
			content = append(content, widget.NewSeparator())
		}
		item := localItems[entry.ItemID]
		if item == nil {
			item = remoteItems[entry.ItemID]
		}
		content = append(content, view.newItemRow(entry, item))
	}
	view.updateCount()
	view.cancel = widget.NewButton("Cancel", cancel)
	footer := container.NewBorder(widget.NewSeparator(), nil, view.cancel, nil, nil)
	view.view = newSecondaryPage(header, footer, container.NewVScroll(container.NewVBox(content...)))
	return view
}

func (v *syncProgressView) newItemRow(entry pawsync.PlanEntry, item *pawsync.ManifestItem) fyne.CanvasObject {
	title := "Untitled item"
	typeName := "Item"
	itemIcon := icons.File
	if item != nil {
		if strings.TrimSpace(item.Title) != "" {
			title = strings.TrimSpace(item.Title)
		}
		if !item.Type.IsZero() {
			typeName = item.Type.Label()
		}
		itemIcon = syncReviewItemIcon(item.Type)
	}
	directionIcon, target := syncConfirmAction(entry.Decision, v.deviceName)
	activity := widget.NewActivity()
	state := newText("Pending", captionStyle)
	row := &syncProgressRow{
		activity: activity,
		status:   state,
	}
	v.rows[entry.ItemID] = row
	info := container.NewVBox(newText(title, itemTitleStyle), newText(typeName, captionStyle))
	transfer := container.NewHBox(widget.NewIcon(directionIcon), newText(target, captionStyle))
	stateWithActivity := container.NewHBox(activity, state)
	return container.NewBorder(nil, nil, navigationRowIcon(itemIcon), stateWithActivity, container.NewBorder(nil, nil, nil, transfer, info))
}

func (v *syncProgressView) Apply(entry pawsync.PlanEntry, started bool, transferErr error) {
	row := v.rows[entry.ItemID]
	if row == nil {
		return
	}
	if started {
		row.started = true
		if entry.Decision == pawsync.Skip {
			return
		}
		row.status.SetText("In progress")
		row.activity.Start()
		row.status.Refresh()
		return
	}
	row.activity.Stop()
	if transferErr != nil {
		row.status.SetText("Failed")
		row.status.Importance = widget.DangerImportance
		row.status.Refresh()
		return
	}
	if entry.Decision == pawsync.Skip {
		row.status.SetText("Skipped")
		row.status.Refresh()
		return
	}
	row.status.SetText("Done")
	row.status.Importance = widget.SuccessImportance
	row.status.Refresh()
	v.completed++
	v.updateCount()
}

func (v *syncProgressView) Finish(result pawsync.ExecutionResult, err error) {
	if v.clone {
		if v.progress != nil {
			v.progress.Stop()
		}
		v.cancel.Disable()
		return
	}
	v.cancel.Disable()
	for _, entry := range result.Unattempted {
		if row := v.rows[entry.ItemID]; row != nil {
			row.activity.Stop()
			row.status.SetText("Not attempted")
			row.status.Refresh()
		}
	}
	if result.Failed != nil {
		if row := v.rows[result.Failed.ItemID]; row != nil {
			row.activity.Stop()
			if errors.Is(err, context.Canceled) {
				row.status.Importance = widget.MediumImportance
				if row.started {
					row.status.SetText("Cancelled")
				} else {
					row.status.SetText("Not started")
				}
			} else if !row.started {
				row.status.SetText("Not started")
			}
			row.status.Refresh()
		}
	}
	if err == nil {
		if v.total == 0 {
			v.setStatus("No operations were needed.", false)
		} else {
			v.setStatus("Sync completed.", false)
		}
		return
	}
	message := fmt.Sprintf("Sync stopped after %d of %d operations.", v.completed, v.total)
	if errors.Is(err, context.Canceled) {
		message = fmt.Sprintf("Sync cancelled after %d of %d operations.", v.completed, v.total)
	}
	failedRow := (*syncProgressRow)(nil)
	if result.Failed != nil {
		failedRow = v.rows[result.Failed.ItemID]
	}
	if failedRow != nil && failedRow.started &&
		(result.Failed.Decision == pawsync.CopyLocalToRemote || result.Failed.Decision == pawsync.DeleteRemote) {
		message += " The current operation may have completed on the other device."
	}
	if !errors.Is(err, context.Canceled) {
		message += " " + err.Error()
	}
	v.setStatus(message, true)
}

func (v *syncProgressView) setStatus(message string, failed bool) {
	v.status.SetText(message)
	if failed {
		v.status.Importance = widget.DangerImportance
	} else {
		v.status.Importance = widget.MediumImportance
	}
	v.status.Refresh()
}

func (v *syncProgressView) updateCount() {
	v.count.SetText(fmt.Sprintf("%d of %d operations completed", v.completed, v.total))
	v.count.Refresh()
}

func syncOperationCount(summary pawsync.PlanSummary) int {
	return summary.CopyLocalToRemote + summary.CopyRemoteToLocal + summary.DeleteLocal + summary.DeleteRemote
}
