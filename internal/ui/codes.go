// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.lucor.dev/cxf"
	"go.lucor.dev/paw/internal/ui/assets/icons"
)

// codesEntry combines an item summary with its TOTP credential.
type codesEntry struct {
	Title, Username string
	ItemID          string
	Icon            fyne.Resource
	Credential      cxf.TOTP
}

type codeEntryOpener interface {
	openCodeEntry(itemID string)
}

// codesView displays the TOTP credentials belonging to the active vault.
type codesView struct {
	entries  []codesEntry
	displays []*TOTPDisplay
	View     fyne.CanvasObject
	opener   codeEntryOpener
}

// newCodesView creates a scrollable, compact list of verification codes.
func newCodesView(ctx context.Context, entries []codesEntry, opener codeEntryOpener) *codesView {
	view := &codesView{entries: entries, opener: opener}
	objects := []fyne.CanvasObject{}
	for index := range view.entries {
		entry := &view.entries[index]
		display, ok := newTOTPDisplay(entry.Credential)
		if !ok {
			continue
		}
		if len(view.displays) > 0 {
			objects = append(objects, widget.NewSeparator())
		}
		view.displays = append(view.displays, display)
		icon := entry.Icon
		if icon == nil {
			icon = icons.Globe
		}
		header := NewActionRow(entry.Title, entry.Username, icon, func() {
			if view.opener != nil {
				view.opener.openCodeEntry(entry.ItemID)
			}
		})
		header.SetFlat(true)
		row := container.NewVBox(header, display.View())
		objects = append(objects, row)
	}
	var content fyne.CanvasObject
	if len(view.displays) == 0 {
		emptyTitle := newText("No verification codes", headingStyle)
		emptyTitle.Alignment = fyne.TextAlignCenter
		emptyDescription := newText("Verification codes from your login items will appear here.", captionStyle)
		emptyDescription.Alignment = fyne.TextAlignCenter
		empty := container.NewVBox(emptyTitle, emptyDescription)
		content = container.NewCenter(empty)
	} else {
		content = container.NewVScroll(container.NewVBox(objects...))
	}
	count := newText(codeCountLabel(len(view.displays)), bodyStyle)
	count.Wrapping = fyne.TextWrapOff
	heading := container.NewBorder(nil, nil, newText("Verification codes", headingStyle), count, nil)
	padding := theme.Size(theme.SizeNamePadding)
	body := container.New(layout.NewCustomPaddedLayout(padding, padding, padding, padding), container.NewBorder(heading, nil, nil, nil, content))
	view.View = body
	if len(view.displays) > 0 {
		go view.run(ctx)
	}
	go func() {
		<-ctx.Done()
		fyne.DoAndWait(view.Clear)
	}()
	return view
}

// Clear removes transient code values and TOTP secrets held by the view.
func (v *codesView) Clear() {
	for index := range v.entries {
		v.entries[index].Credential.Secret = ""
	}
	for _, display := range v.displays {
		display.Clear()
	}
}

func (v *codesView) run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fyne.Do(func() {
				if ctx.Err() == nil {
					for _, display := range v.displays {
						display.Update()
					}
				}
			})
		}
	}
}

func codeCountLabel(count int) string {
	if count == 1 {
		return "1 code"
	}
	return fmt.Sprintf("%d codes", count)
}
