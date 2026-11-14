// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type syncConfirmView struct {
	view   fyne.CanvasObject
	sync   *widget.Button
	plan   *pawsync.Plan
	status *widget.Label
}

func newSyncConfirmView(
	deviceName string,
	localVault, remoteVault pawsync.VaultReference,
	local, remote *pawsync.Manifest,
	plan *pawsync.Plan,
	back, syncPlan, done func(),
) *syncConfirmView {
	backButton := widget.NewButtonWithIcon("", icons.Back, back)
	backButton.Importance = widget.LowImportance
	header := newPageHeader(backButton, newText("Sync", titleStyle), nil)
	view := &syncConfirmView{
		plan:   plan,
		status: newText("", captionStyle),
	}
	summary := plan.Summary()
	operationCount := syncExecutableCount(summary)
	content := []fyne.CanvasObject{}
	var pageContent fyne.CanvasObject
	if operationCount == 0 {
		title, message := "Already in sync", "Both vaults are up to date."
		if summary.Skipped > 0 {
			title = "No changes selected"
			message = syncSkippedSummary(summary.Skipped)
		}
		pageContent = container.NewVBox(
			layout.NewSpacer(),
			container.NewCenter(newText(title, headingStyle)),
			wrappedCenteredText(message, bodyStyle),
			layout.NewSpacer(),
		)
	} else {
		content = append(content, container.NewHBox(navigationRowIcon(icons.Laptop), newText("Ready to sync with "+deviceName, bodyStyle)))
		content = append(content, syncConfirmSummary(summary, deviceName)...)
		if summary.Skipped > 0 {
			content = append(content, newText(syncSkippedSummary(summary.Skipped), captionStyle))
		}
		content = append(content, widget.NewSeparator(), newText(fmt.Sprintf("Items (%d)", operationCount), sectionTitleStyle))
		localItems := manifestItemsByID(local)
		remoteItems := manifestItemsByID(remote)
		visibleEntries := 0
		for _, entry := range plan.Entries {
			if entry.Decision == pawsync.Skip {
				continue
			}
			if visibleEntries > 0 {
				content = append(content, widget.NewSeparator())
			}
			content = append(content, syncConfirmItemRow(entry, localItems[entry.ItemID], remoteItems[entry.ItemID], deviceName))
			visibleEntries++
		}
		pageContent = container.NewVScroll(container.NewVBox(content...))
	}
	backAction := widget.NewButton("Back", back)
	buttonText, action := "Sync", syncPlan
	if operationCount == 0 {
		buttonText, action = "Done", done
	}
	view.sync = widget.NewButton(buttonText, action)
	view.sync.Importance = widget.HighImportance
	view.updateReadiness()
	footer := container.NewBorder(widget.NewSeparator(), nil, backAction, view.sync, view.status)
	view.view = newSecondaryPage(header, footer, pageContent)
	return view
}

func syncConfirmSummary(summary pawsync.PlanSummary, deviceName string) []fyne.CanvasObject {
	var rows []fyne.CanvasObject
	appendRow := func(text string, count int) {
		if count == 0 {
			return
		}
		rows = append(rows, container.NewBorder(nil, nil,
			nil,
			newText(fmt.Sprintf("%d", count), bodyStyle),
			newText(text+":", bodyStyle),
		))
	}
	appendRow("Copy to "+deviceName, summary.CopyLocalToRemote)
	appendRow("Copy to this device", summary.CopyRemoteToLocal)
	appendRow("Delete from this device", summary.DeleteLocal)
	appendRow("Delete from "+deviceName, summary.DeleteRemote)
	return rows
}

func syncExecutableCount(summary pawsync.PlanSummary) int {
	return summary.CopyLocalToRemote + summary.CopyRemoteToLocal + summary.DeleteLocal + summary.DeleteRemote
}

func syncSkippedSummary(count int) string {
	label := "items"
	if count == 1 {
		label = "item"
	}
	return fmt.Sprintf("%d %s skipped.", count, label)
}

func syncConfirmItemRow(
	entry pawsync.PlanEntry,
	local, remote *pawsync.ManifestItem,
	deviceName string,
) fyne.CanvasObject {
	item := local
	if item == nil {
		item = remote
	}
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
	_, action := syncConfirmAction(entry.Decision, deviceName)
	itemDetails := container.New(layout.NewCustomPaddedVBoxLayout(0),
		newText(title, itemTitleStyle),
		newText(typeName, captionStyle),
		newText(action, captionStyle),
	)
	return container.NewBorder(nil, nil, navigationRowIcon(itemIcon), nil, itemDetails)
}

func syncConfirmAction(decision pawsync.Decision, deviceName string) (fyne.Resource, string) {
	switch decision {
	case pawsync.CopyLocalToRemote:
		return icons.Next, "Copy to " + deviceName
	case pawsync.CopyRemoteToLocal:
		return icons.Back, "Copy to this device"
	case pawsync.DeleteLocal:
		return icons.Delete, "Delete from this device"
	case pawsync.DeleteRemote:
		return icons.Delete, "Delete from " + deviceName
	case pawsync.Skip:
		return icons.Minus, "Skip"
	default:
		return icons.Info, "Decision required"
	}
}

func (v *syncConfirmView) updateReadiness() {
	if v.plan == nil {
		v.sync.Disable()
		return
	}
	ready, err := v.plan.Ready()
	if err != nil || !ready || v.sync.OnTapped == nil {
		v.sync.Disable()
		return
	}
	v.sync.Enable()
}
