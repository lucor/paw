// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type syncReceiverProgressView struct {
	view       fyne.CanvasObject
	header     fyne.CanvasObject
	activity   *widget.Activity
	progress   *widget.ProgressBarInfinite
	heading    *widget.Label
	count      *widget.Label
	status     *widget.Label
	items      *fyne.Container
	cancel     *widget.Button
	rows       map[string]*syncReceiverProgressRow
	completed  int
	deviceName string
	clone      bool
}

type syncReceiverProgressRow struct {
	activity *widget.Activity
	status   *widget.Label
}

func newSyncReceiverProgressView(deviceName string, cancel func(), clone bool) *syncReceiverProgressView {
	back := widget.NewButtonWithIcon("", icons.Back, cancel)
	back.Importance = widget.LowImportance
	title := "Sync"
	activityText := "Receiving changes…"
	statusText := "Syncing with " + deviceName + "."
	if clone {
		title = "Clone"
		activityText = "Transferring vault data…"
		statusText = "Sending to " + deviceName
	}
	header := newPageHeader(back, newText(title, titleStyle), nil)
	view := &syncReceiverProgressView{
		header:     header,
		activity:   widget.NewActivity(),
		progress:   widget.NewProgressBarInfinite(),
		heading:    newText(activityText, headingStyle),
		count:      newText("Waiting for incoming changes…", sectionTitleStyle),
		status:     newText("", captionStyle),
		items:      container.NewVBox(),
		rows:       make(map[string]*syncReceiverProgressRow),
		deviceName: deviceName,
		clone:      clone,
	}
	view.activity.Start()
	view.progress.Start()
	content := container.NewVBox(
		container.NewCenter(view.activity),
		container.NewCenter(view.heading),
		container.NewCenter(newText(statusText, sectionTitleStyle)),
		view.progress,
		container.NewCenter(view.count),
		view.status,
		view.items,
	)
	view.cancel = widget.NewButton("Cancel", cancel)
	footer := container.NewBorder(widget.NewSeparator(), nil, view.cancel, nil, nil)
	view.view = newSecondaryPage(header, footer, container.NewVScroll(content))
	return view
}

// Apply adds or updates an item received from the trusted peer.
func (v *syncReceiverProgressView) Apply(event pawsync.SyncProgressEvent) {
	if v.clone {
		return
	}
	row := v.rows[event.ItemID]
	if row == nil {
		row = v.newItemRow(event)
		v.rows[event.ItemID] = row
	}
	if event.Started {
		row.status.SetText("In progress")
		row.activity.Start()
		row.status.Refresh()
		return
	}
	row.activity.Stop()
	if event.Failed {
		row.status.SetText("Failed")
		row.status.Importance = widget.DangerImportance
		row.status.Refresh()
		return
	}
	if !event.Completed {
		return
	}
	row.status.SetText("Done")
	row.status.Importance = widget.SuccessImportance
	row.status.Refresh()
	v.completed++
	v.count.SetText(fmt.Sprintf("%d operations received", v.completed))
	v.count.Refresh()
}

// Finish reports the peer's completion outcome without guessing an unknown plan total.
func (v *syncReceiverProgressView) Finish(successful bool) {
	v.activity.Stop()
	v.progress.Stop()
	if successful {
		check := container.NewGridWrap(
			fyne.NewSquareSize(pawSize(pawSizeNameDetailIconArea)),
			container.NewCenter(widget.NewIcon(icons.CircleCheck)),
		)
		completed := "No operations were received."
		if v.completed == 1 {
			completed = "1 operation completed."
		} else if v.completed > 1 {
			completed = fmt.Sprintf("%d operations completed.", v.completed)
		}
		completedCount := v.completed
		content := []fyne.CanvasObject{
			container.NewCenter(check),
			container.NewCenter(newText("Sync completed", headingStyle)),
			container.NewCenter(newText(completed, sectionTitleStyle)),
		}
		if completedCount > 0 {
			content = append(content, widget.NewSeparator())
			content = append(content, container.NewBorder(
				nil,
				nil,
				nil,
				newText(fmt.Sprint(completedCount), itemTitleStyle),
				newText("Received from "+v.deviceName, bodyStyle),
			))
		}
		v.cancel.SetText("Done")
		v.cancel.Importance = widget.HighImportance
		v.cancel.Refresh()
		footer := container.NewBorder(widget.NewSeparator(), nil, nil, v.cancel, nil)
		v.view = newSecondaryPage(v.header, footer, container.NewVScroll(container.NewVBox(content...)))
		v.activity = nil
		v.progress = nil
		v.heading = nil
		v.count = nil
		v.status = nil
		v.items = nil
		v.rows = nil
		v.completed = 0
		return
	}
	v.heading.SetText("Sync failed")
	v.count.SetText("The other device did not complete Sync.")
	v.status.SetText("Run Sync again to verify the state of your vaults.")
	v.count.Refresh()
	v.status.Refresh()
	v.cancel.SetText("Done")
	v.cancel.Importance = widget.HighImportance
	v.cancel.Refresh()
}

func (v *syncReceiverProgressView) newItemRow(event pawsync.SyncProgressEvent) *syncReceiverProgressRow {
	title := event.Title
	if title == "" {
		title = "Untitled item"
	}
	typeName := "Item"
	itemIcon := icons.File
	if !event.Type.IsZero() {
		typeName = event.Type.Label()
		itemIcon = syncReviewItemIcon(event.Type)
	}
	activity := widget.NewActivity()
	status := newText("Pending", captionStyle)
	details := container.NewVBox(newText(title, itemTitleStyle), newText(typeName, captionStyle))
	row := container.NewBorder(
		nil,
		widget.NewSeparator(),
		navigationRowIcon(itemIcon),
		container.NewHBox(activity, status),
		details,
	)
	v.items.Add(row)
	return &syncReceiverProgressRow{activity: activity, status: status}
}
