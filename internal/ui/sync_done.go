// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type syncDoneView struct {
	view fyne.CanvasObject
	done *widget.Button
}

func newSyncDoneView(deviceName string, summary pawsync.PlanSummary, note string, done func()) *syncDoneView {
	return newDoneView("Sync", "Sync completed", deviceName, summary, note, done)
}

func newCloneDoneView(info pawsync.VaultReference, openVault func()) *syncDoneView {
	back := widget.NewButtonWithIcon("", icons.Back, openVault)
	back.Importance = widget.LowImportance
	header := newPageHeader(back, newText("Clone", titleStyle), nil)
	check := container.NewGridWrap(fyne.NewSquareSize(pawSize(pawSizeNameDetailIconArea)), widget.NewIcon(icons.CircleCheck))
	content := []fyne.CanvasObject{
		container.NewCenter(check),
		container.NewCenter(newText("Clone completed", headingStyle)),
		container.NewCenter(wrappedCenteredText(fmt.Sprintf("Vault %q is ready on this device.", info.Name), bodyStyle)),
	}
	if summary := cloneDoneSummary(info); summary != "" {
		content = append(content, container.NewCenter(wrappedCenteredText(summary, bodyStyle)))
	}
	view := &syncDoneView{}
	view.done = widget.NewButton("Open vault", openVault)
	view.done.Importance = widget.HighImportance
	footer := container.NewBorder(widget.NewSeparator(), nil, nil, view.done, nil)
	view.view = newSecondaryPage(header, footer, container.NewVScroll(container.NewVBox(content...)))
	return view
}

func cloneDoneSummary(info pawsync.VaultReference) string {
	var parts []string
	if info.Items != 0 {
		label := "items"
		if info.Items == 1 {
			label = "item"
		}
		parts = append(parts, fmt.Sprintf("%d %s", info.Items, label))
	}
	if info.Attachments != 0 {
		label := "attachments"
		if info.Attachments == 1 {
			label = "attachment"
		}
		parts = append(parts, fmt.Sprintf("%d %s", info.Attachments, label))
	}
	return strings.Join(parts, " · ")
}

func newCloneSourceDoneView(deviceName string, done func()) *syncDoneView {
	back := widget.NewButtonWithIcon("", icons.Back, done)
	back.Importance = widget.LowImportance
	header := newPageHeader(back, newText("Clone", titleStyle), nil)
	check := container.NewGridWrap(fyne.NewSquareSize(pawSize(pawSizeNameDetailIconArea)), widget.NewIcon(icons.CircleCheck))
	content := []fyne.CanvasObject{
		container.NewCenter(check),
		container.NewCenter(newText("Clone completed", headingStyle)),
		container.NewCenter(wrappedCenteredText("The vault has been transferred to "+deviceName+".", bodyStyle)),
	}
	view := &syncDoneView{}
	view.done = widget.NewButton("Done", done)
	view.done.Importance = widget.HighImportance
	footer := container.NewBorder(widget.NewSeparator(), nil, nil, view.done, nil)
	view.view = newSecondaryPage(header, footer, container.NewVScroll(container.NewVBox(content...)))
	return view
}

func newDoneView(pageTitle, title, deviceName string, summary pawsync.PlanSummary, note string, done func()) *syncDoneView {
	back := widget.NewButtonWithIcon("", icons.Back, done)
	back.Importance = widget.LowImportance
	header := newPageHeader(back, newText(pageTitle, titleStyle), nil)
	check := container.NewGridWrap(
		fyne.NewSquareSize(pawSize(pawSizeNameDetailIconArea)),
		container.NewCenter(widget.NewIcon(icons.CircleCheck)),
	)
	operationCount := syncOperationCount(summary)
	completed := fmt.Sprintf("%d operations completed.", operationCount)
	if operationCount == 1 {
		completed = "1 operation completed."
	} else if operationCount == 0 {
		completed = "No operations were needed."
	}
	content := []fyne.CanvasObject{
		container.NewCenter(check),
		container.NewCenter(newText(title, headingStyle)),
		container.NewCenter(newText(completed, sectionTitleStyle)),
	}
	rows := syncDoneSummary(summary, deviceName)
	if len(rows) > 0 {
		content = append(content, widget.NewSeparator())
		content = append(content, rows...)
	}
	if note != "" {
		content = append(content, newText(note, captionStyle))
	}
	view := &syncDoneView{}
	view.done = widget.NewButton("Done", done)
	view.done.Importance = widget.HighImportance
	footer := container.NewBorder(widget.NewSeparator(), nil, nil, view.done, nil)
	view.view = newSecondaryPage(header, footer, container.NewVScroll(container.NewVBox(content...)))
	return view
}

func syncDoneSummary(summary pawsync.PlanSummary, deviceName string) []fyne.CanvasObject {
	var rows []fyne.CanvasObject
	appendRow := func(label string, count int) {
		if count == 0 {
			return
		}
		rows = append(rows, container.NewBorder(
			nil,
			nil,
			nil,
			newText(fmt.Sprint(count), itemTitleStyle),
			newText(label, bodyStyle),
		))
	}
	appendRow("Copied to "+deviceName, summary.CopyLocalToRemote)
	appendRow("Copied to this device", summary.CopyRemoteToLocal)
	appendRow("Deleted", summary.DeleteLocal+summary.DeleteRemote)
	appendRow("Skipped", summary.Skipped)
	return rows
}
