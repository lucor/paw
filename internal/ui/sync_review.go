// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/model"
	pawsync "go.lucor.dev/paw/internal/sync"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type syncReviewView struct {
	view        fyne.CanvasObject
	status      *widget.Label
	progress    *widget.Label
	review      *widget.Button
	plan        *pawsync.Plan
	actions     map[string]*widget.Select
	localVault  pawsync.VaultReference
	remoteVault pawsync.VaultReference
	local       *pawsync.Manifest
	remote      *pawsync.Manifest
	deviceName  string
}

type syncReviewOption struct {
	label    string
	decision pawsync.Decision
}

func newSyncReviewView(
	deviceName string,
	localVault, remoteVault pawsync.VaultReference,
	diff []pawsync.Difference,
	local, remote *pawsync.Manifest,
	plan *pawsync.Plan,
	cancel, review func(),
) *syncReviewView {
	back := widget.NewButtonWithIcon("", icons.Back, cancel)
	back.Importance = widget.LowImportance
	header := newPageHeader(back, newText("Sync", titleStyle), nil)
	identical := 0
	for _, difference := range diff {
		if difference.Kind == pawsync.Identical {
			identical++
		}
	}
	itemsToReview := 0
	if plan != nil {
		itemsToReview = len(plan.Entries)
	}
	identicalLabel, reviewLabel := "items", "items"
	if identical == 1 {
		identicalLabel = "item"
	}
	if itemsToReview == 1 {
		reviewLabel = "item"
	}
	syncLabel := newText("Sync with", bodyStyle)
	syncLabel.Wrapping = fyne.TextWrapOff
	deviceLabel := newText(deviceName, itemTitleStyle)
	deviceLabel.Wrapping = fyne.TextWrapOff
	intro := container.NewVBox(
		container.NewBorder(nil, nil, navigationRowIcon(icons.Laptop), nil, container.NewVBox(
			container.NewHBox(syncLabel, deviceLabel),
			newText(fmt.Sprintf("%d identical %s · %d %s to review", identical, identicalLabel, itemsToReview, reviewLabel), captionStyle),
		)),
		widget.NewSeparator(),
	)
	localItems := manifestItemsByID(local)
	remoteItems := manifestItemsByID(remote)
	view := &syncReviewView{
		plan:        plan,
		actions:     make(map[string]*widget.Select),
		status:      newText("", captionStyle),
		progress:    newText("", captionStyle),
		localVault:  localVault,
		remoteVault: remoteVault,
		local:       local,
		remote:      remote,
		deviceName:  deviceName,
	}
	view.status.Hide()
	if itemsToReview == 0 {
		done := widget.NewButton("Done", review)
		done.Importance = widget.HighImportance
		view.review = done
		footer := container.NewBorder(widget.NewSeparator(), nil, nil, done, nil)
		content := container.NewVBox(
			layout.NewSpacer(),
			container.NewCenter(newText("Already in sync", headingStyle)),
			wrappedCenteredText("Both vaults are up to date.", bodyStyle),
			layout.NewSpacer(),
		)
		view.view = newSecondaryPage(header, footer, content)
		return view
	}
	rows := []fyne.CanvasObject{intro}
	if plan != nil {
		for index, entry := range plan.Entries {
			if index > 0 {
				rows = append(rows, widget.NewSeparator())
			}
			localItem := localItems[entry.ItemID]
			remoteItem := remoteItems[entry.ItemID]
			rows = append(rows, view.newItemRow(deviceName, entry, localItem, remoteItem))
		}
	}
	view.review = widget.NewButton("Review plan", review)
	view.review.Importance = widget.HighImportance
	view.updateReadiness()
	cancelButton := widget.NewButton("Cancel", cancel)
	footer := container.NewVBox(widget.NewSeparator(), view.progress, view.status, container.NewBorder(nil, nil, cancelButton, view.review, nil))
	view.view = newSecondaryPage(header, footer, container.NewVScroll(container.NewVBox(rows...)))
	return view
}

func (v *syncReviewView) newItemRow(
	deviceName string,
	entry pawsync.PlanEntry,
	local, remote *pawsync.ManifestItem,
) fyne.CanvasObject {
	item := local
	if item == nil {
		item = remote
	}
	title := "Untitled item"
	typeName := "Item"
	icon := icons.File
	if item != nil {
		if strings.TrimSpace(item.Title) != "" {
			title = strings.TrimSpace(item.Title)
		}
		if !item.Type.IsZero() {
			typeName = item.Type.Label()
		}
		icon = syncReviewItemIcon(item.Type)
	}
	status, _ := syncReviewItemDescription(entry.Kind, deviceName, local, remote)
	info := container.New(layout.NewCustomPaddedVBoxLayout(0), newText(title, itemTitleStyle), newText(typeName+" · "+status, captionStyle))
	if entry.Kind == pawsync.ContentDifferent || entry.Kind == pawsync.StateDifferent {
		localTime := newText(syncReviewTimestamp(local), captionStyle)
		localTime.Wrapping = fyne.TextWrapOff
		remoteTime := newText(syncReviewTimestamp(remote), captionStyle)
		remoteTime.Wrapping = fyne.TextWrapOff
		info.Add(container.NewBorder(nil, nil, newText("This device", captionStyle), localTime, nil))
		info.Add(container.NewBorder(nil, nil, newText(deviceName, captionStyle), remoteTime, nil))
	}
	options := syncReviewOptions(entry.Kind, deviceName)
	if (entry.Kind == pawsync.ContentDifferent || entry.Kind == pawsync.StateDifferent) &&
		local != nil && remote != nil && local.ModifiedAt != nil && remote.ModifiedAt != nil &&
		*local.ModifiedAt > 0 && *remote.ModifiedAt > 0 &&
		*local.ModifiedAt <= math.MaxInt64 && *remote.ModifiedAt <= math.MaxInt64 &&
		*local.ModifiedAt != *remote.ModifiedAt {
		latest := pawsync.CopyLocalToRemote
		if *remote.ModifiedAt > *local.ModifiedAt {
			latest = pawsync.CopyRemoteToLocal
		}
		for index := range options {
			if options[index].decision == latest {
				options[index].label += " (Latest)"
			}
		}
	}
	labels := []string{"Choose action"}
	decisions := map[string]pawsync.Decision{"Choose action": pawsync.RequiresDecision}
	for _, option := range options {
		labels = append(labels, option.label)
		decisions[option.label] = option.decision
	}
	selectAction := widget.NewSelect(labels, nil)
	selectAction.SetSelected("Choose action")
	selectAction.OnChanged = func(selected string) {
		decision, ok := decisions[selected]
		if !ok {
			return
		}
		if err := v.plan.SetDecision(entry.ItemID, decision); err != nil {
			v.SetStatus(err.Error(), true)
			return
		}
		v.SetStatus("", false)
		v.updateReadiness()
	}
	v.actions[entry.ItemID] = selectAction
	info.Add(selectAction)
	return container.NewBorder(nil, nil, container.NewVBox(navigationRowIcon(icon)), nil, info)
}

func (v *syncReviewView) updateReadiness() {
	if v.plan == nil {
		v.progress.SetText("0 of 0 reviewed")
		v.review.Disable()
		return
	}
	reviewed := 0
	for _, entry := range v.plan.Entries {
		if entry.Decision != pawsync.RequiresDecision {
			reviewed++
		}
	}
	v.progress.SetText(fmt.Sprintf("%d of %d reviewed", reviewed, len(v.plan.Entries)))
	ready, err := v.plan.Ready()
	if err != nil || !ready {
		v.review.Disable()
		return
	}
	if v.review.OnTapped == nil {
		v.review.Disable()
		return
	}
	v.review.Enable()
}

func (v *syncReviewView) SetStatus(message string, failed bool) {
	v.status.SetText(message)
	if message == "" {
		v.status.Hide()
	} else {
		v.status.Show()
	}
	if failed {
		v.status.Importance = widget.DangerImportance
	} else {
		v.status.Importance = widget.MediumImportance
	}
	v.status.Refresh()
}

func manifestItemsByID(manifest *pawsync.Manifest) map[string]*pawsync.ManifestItem {
	items := make(map[string]*pawsync.ManifestItem)
	if manifest == nil {
		return items
	}
	for index := range manifest.Items {
		items[manifest.Items[index].ID] = &manifest.Items[index]
	}
	return items
}

func syncReviewItemIcon(itemType model.ItemType) fyne.Resource {
	switch itemType {
	case model.LoginItemType:
		return icons.Login
	case model.PasswordItemType:
		return icons.Password
	case model.NoteItemType:
		return icons.Note
	case model.SSHKeyItemType:
		return icons.SSH
	default:
		return icons.File
	}
}

func syncReviewItemDescription(
	kind pawsync.DifferenceKind,
	deviceName string,
	local, remote *pawsync.ManifestItem,
) (string, string) {
	switch kind {
	case pawsync.LocalOnly:
		return "Only on this device", "Modified: " + syncReviewTimestamp(local)
	case pawsync.RemoteOnly:
		return "Only on " + deviceName, "Modified: " + syncReviewTimestamp(remote)
	case pawsync.ContentDifferent, pawsync.StateDifferent:
		return "Different versions", fmt.Sprintf(
			"This device: %s\n%s: %s",
			syncReviewTimestamp(local),
			deviceName,
			syncReviewTimestamp(remote),
		)
	default:
		return "", ""
	}
}

func syncReviewTimestamp(item *pawsync.ManifestItem) string {
	if item == nil || item.ModifiedAt == nil || *item.ModifiedAt == 0 || *item.ModifiedAt > math.MaxInt64 {
		return "Unknown"
	}
	return time.Unix(int64(*item.ModifiedAt), 0).Local().Format("Jan 2, 2006, 15:04")
}

func syncReviewOptions(kind pawsync.DifferenceKind, deviceName string) []syncReviewOption {
	switch kind {
	case pawsync.LocalOnly:
		return []syncReviewOption{
			{
				label:    "Copy to " + deviceName,
				decision: pawsync.CopyLocalToRemote,
			},
			{
				label:    "Delete from this device",
				decision: pawsync.DeleteLocal,
			},
			{
				label:    "Skip",
				decision: pawsync.Skip,
			},
		}
	case pawsync.RemoteOnly:
		return []syncReviewOption{
			{
				label:    "Copy to this device",
				decision: pawsync.CopyRemoteToLocal,
			},
			{
				label:    "Delete from " + deviceName,
				decision: pawsync.DeleteRemote,
			},
			{
				label:    "Skip",
				decision: pawsync.Skip,
			},
		}
	case pawsync.ContentDifferent, pawsync.StateDifferent:
		return []syncReviewOption{
			{
				label:    "Keep this device’s version",
				decision: pawsync.CopyLocalToRemote,
			},
			{
				label:    "Keep " + deviceName + "’s version",
				decision: pawsync.CopyRemoteToLocal,
			},
			{
				label:    "Skip",
				decision: pawsync.Skip,
			},
		}
	default:
		return nil
	}
}
