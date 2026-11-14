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

type syncFailedView struct {
	view fyne.CanvasObject
	done *widget.Button
}

func newSyncFailedView(deviceName string, plan *pawsync.Plan, result pawsync.ExecutionResult, done func()) *syncFailedView {
	recovery := "Run Sync again to verify the state of your vaults."
	if result.Failed != nil && (result.Failed.Decision == pawsync.CopyLocalToRemote || result.Failed.Decision == pawsync.DeleteRemote) {
		recovery = "The current operation may have completed on the other device. " + recovery
	}
	return newFailedView("Sync", "Sync failed", "The vaults may contain partial changes.", recovery, deviceName, plan, result, true, done)
}

func newCloneFailedView(deviceName string, plan *pawsync.Plan, result pawsync.ExecutionResult, cleanupFailed, pairingRemoved bool, done func()) *syncFailedView {
	summary := "The clone from " + deviceName + " did not complete."
	var recovery []string
	if cleanupFailed {
		recovery = append(recovery, "Paw could not remove the incomplete vault.")
	} else {
		recovery = append(recovery, "The incomplete vault was removed.")
	}
	if pairingRemoved {
		recovery = append(recovery, "The temporary pairing was removed.")
	} else {
		recovery = append(recovery, "The Source may still have the temporary pairing.")
	}
	return newFailedView("Clone", "Clone failed", summary, strings.Join(recovery, " "), deviceName, plan, result, false, done)
}

func newCloneSourceFailedView(deviceName string, done func()) *syncFailedView {
	plan := &pawsync.Plan{}
	return newFailedView("Clone", "Clone failed", "The clone to "+deviceName+" did not complete.", "The temporary pairing was removed.", deviceName, plan, pawsync.ExecutionResult{}, false, done)
}

func newCloneSourceUnknownView(deviceName string, done func()) *syncFailedView {
	return newFailedView("Clone", "Connection lost", "The clone outcome is unknown.", "Check the other device. The pairing may still be present.", deviceName, &pawsync.Plan{}, pawsync.ExecutionResult{}, false, done)
}

func newSyncConnectionLostView(deviceName string, done func()) *syncFailedView {
	return newFailedView("Sync", "Connection lost", "The sync outcome is unknown.", "Run Sync again to verify the state of your vaults.", deviceName, &pawsync.Plan{}, pawsync.ExecutionResult{}, false, done)
}

func newSyncCancelledView(deviceName string, done func()) *syncFailedView {
	return newFailedView("Sync", "Sync cancelled", "The other device cancelled Sync.", "Run Sync again to verify the state of your vaults.", deviceName, &pawsync.Plan{}, pawsync.ExecutionResult{}, false, done)
}

func newFailedView(pageTitle, title, summary, recoveryMessage, deviceName string, plan *pawsync.Plan, result pawsync.ExecutionResult, showOperations bool, done func()) *syncFailedView {
	back := widget.NewButtonWithIcon("", icons.Back, done)
	back.Importance = widget.LowImportance
	header := newPageHeader(back, newText(pageTitle, titleStyle), nil)
	errorIcon := container.NewGridWrap(
		fyne.NewSquareSize(pawSize(pawSizeNameDetailIconArea)),
		container.NewCenter(widget.NewIcon(icons.CircleX)),
	)
	if showOperations {
		total := syncOperationCount(plan.Summary())
		completed := 0
		for _, entry := range result.Completed {
			if entry.Decision != pawsync.Skip {
				completed++
			}
		}
		summary = fmt.Sprintf("%d of %d operations completed.", completed, total)
	}
	content := []fyne.CanvasObject{
		container.NewCenter(errorIcon),
		container.NewCenter(newText(title, headingStyle)),
		container.NewCenter(wrappedCenteredText(summary, sectionTitleStyle)),
	}
	completedIDs := make(map[string]bool, len(result.Completed))
	for _, entry := range result.Completed {
		completedIDs[entry.ItemID] = true
	}
	unattemptedIDs := make(map[string]bool, len(result.Unattempted))
	for _, entry := range result.Unattempted {
		unattemptedIDs[entry.ItemID] = true
	}
	rows := make([]fyne.CanvasObject, 0, len(plan.Entries))
	for _, entry := range plan.Entries {
		if !showOperations {
			break
		}
		if entry.Decision == pawsync.Skip {
			continue
		}
		icon := icons.Timer
		if completedIDs[entry.ItemID] {
			icon = icons.Check
		} else if !unattemptedIDs[entry.ItemID] {
			icon = icons.Bug
		}
		direction, _ := syncConfirmAction(entry.Decision, deviceName)
		label := "Copied to " + deviceName
		switch entry.Decision {
		case pawsync.CopyRemoteToLocal:
			label = "Copied to this device"
		case pawsync.DeleteLocal:
			label = "Deleted from this device"
		case pawsync.DeleteRemote:
			label = "Deleted from " + deviceName
		}
		rows = append(rows, container.NewBorder(
			nil,
			widget.NewSeparator(),
			widget.NewIcon(direction),
			widget.NewIcon(icon),
			newText(label, bodyStyle),
		))
	}
	if len(rows) > 0 {
		content = append(content, widget.NewSeparator())
		content = append(content, rows...)
	}
	content = append(content, wrappedCenteredText(recoveryMessage, bodyStyle))
	view := &syncFailedView{}
	view.done = widget.NewButton("Done", done)
	view.done.Importance = widget.HighImportance
	footer := container.NewBorder(widget.NewSeparator(), nil, nil, view.done, nil)
	view.view = newSecondaryPage(header, footer, container.NewVScroll(container.NewVBox(content...)))
	return view
}

func wrappedCenteredText(value string, style textStyle) *widget.Label {
	label := newText(value, style)
	label.Alignment = fyne.TextAlignCenter
	label.Wrapping = fyne.TextWrapWord
	label.Truncation = fyne.TextTruncateOff
	return label
}
