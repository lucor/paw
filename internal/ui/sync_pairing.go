// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

type syncPairingView struct {
	view     fyne.CanvasObject
	status   *widget.Label
	activity *widget.Activity
}

func newSyncPairingView(deviceName, code string, receiver bool, back, confirm func()) *syncPairingView {
	backButton := widget.NewButtonWithIcon("", icons.Back, func() {
		if back != nil {
			back()
		}
	})
	backButton.Importance = widget.LowImportance
	pageHeader := newPageHeader(backButton, newText("Pairing", titleStyle), nil)
	verb := "Pairing with "
	instruction := "Confirm this code on " + deviceName + "."
	if receiver {
		verb = "Pair with "
		instruction = "Does this code match the one on " + deviceName + "?"
	}
	deviceNameText := verb + deviceName
	device := newText(deviceNameText, headingStyle)
	codeText := newText(formatSyncPairingCode(code), headingStyle)
	codeText.Alignment = fyne.TextAlignCenter
	codePanel := container.NewStack(
		canvas.NewRectangle(theme.Color(theme.ColorNameInputBackground)),
		container.NewCenter(codeText),
	)
	statusText := "Waiting for confirmation…"
	status := newText(statusText, captionStyle)
	status.Alignment = fyne.TextAlignLeading
	var activity *widget.Activity
	var waitState fyne.CanvasObject
	if receiver {
		statusText = ""
		waitState = status
	} else {
		activity = widget.NewActivity()
		waitState = container.NewHBox(activity, status)
	}
	contentItems := []fyne.CanvasObject{
		device,
		newText(instruction, bodyStyle),
		container.New(layout.NewCustomPaddedVBoxLayout(pawSize(pawSizeNameFormSpacing)), codePanel),
	}
	if receiver {
		contentItems = append(contentItems, newText("A device wants to connect to this vault.", bodyStyle))
	} else {
		contentItems = append(contentItems, waitState)
	}
	var actions fyne.CanvasObject
	if receiver {
		var decline, confirmButton *widget.Button
		decline = widget.NewButton("Decline", func() {
			if back != nil {
				back()
			}
		})
		confirmButton = widget.NewButton("Confirm pairing", func() {
			confirmButton.Disable()
			if confirm != nil {
				confirm()
			}
		})
		confirmButton.Importance = widget.HighImportance
		actions = container.NewBorder(nil, nil, decline, confirmButton)
	} else {
		cancel := widget.NewButton("Cancel", back)
		actions = container.NewBorder(nil, nil, nil, cancel)
	}
	view := newSecondaryPage(pageHeader, actions, container.NewVScroll(
		container.New(layout.NewCustomPaddedVBoxLayout(pawSize(pawSizeNameFormSpacing)), container.NewVBox(contentItems...)),
	))
	return &syncPairingView{view: view, status: status, activity: activity}
}

func (v *syncPairingView) Start() {
	if v.activity != nil {
		v.activity.Start()
	}
}

func (v *syncPairingView) Stop() {
	if v.activity != nil {
		v.activity.Stop()
	}
}

func (v *syncPairingView) SetStatus(message string, failed bool) {
	v.status.SetText(message)
	if failed {
		v.status.Importance = widget.DangerImportance
	} else {
		v.status.Importance = widget.MediumImportance
	}
	v.status.Refresh()
}

func formatSyncPairingCode(code string) string {
	if len(code) != 6 {
		return code
	}
	return code[:3] + " " + code[3:]
}
