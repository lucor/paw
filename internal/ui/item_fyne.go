// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ui

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"lucor.dev/paw/internal/icon"
	"lucor.dev/paw/internal/paw"
	pawwidget "lucor.dev/paw/internal/widget"
)

// FyneItem wraps all methods allow to handle an Item as Fyne canvas object
type FyneItem interface {
	// Icon returns a fyne resource associated to the imte
	Icon() fyne.Resource
	// Show returns a fyne CanvasObject used to view the item
	Show(ctx context.Context, w fyne.Window) fyne.CanvasObject
	// Edit returns a fyne CanvasObject used to edit the item
	Edit(ctx context.Context, key *paw.Key, w fyne.Window) (fyne.CanvasObject, paw.Item)
	// Item returns the paw Item
	Item() paw.Item
}

// FynePasswordGenerator wraps all methods to show a Fyne dialog to generate passwords
type FynePasswordGenerator interface {
	ShowPasswordGenerator(bind binding.String, password *paw.Password, w fyne.Window)
}

func NewFyneItem(item paw.Item, config *paw.Config) FyneItem {
	var fyneItem FyneItem
	switch item.GetMetadata().Type {
	case paw.NoteItemType:
		fyneItem = &Note{Note: item.(*paw.Note)}
	case paw.LoginItemType:
		fyneItem = &Login{Login: item.(*paw.Login), Config: config}
	case paw.PasswordItemType:
		fyneItem = &Password{Password: item.(*paw.Password), Config: config}
	case paw.SSHKeyItemType:
		fyneItem = &SSHKey{SSHKey: item.(*paw.SSHKey)}
	}
	return fyneItem
}

func titleRow(icon fyne.Resource, text string) []fyne.CanvasObject {
	t := canvas.NewText(text, theme.ForegroundColor())
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextHeadingSize()
	i := widget.NewIcon(icon)
	i.Resize(fyne.NewSize(32, 32))
	return []fyne.CanvasObject{
		i,
		t,
	}
}

func labelWithStyle(label string) *widget.Label {
	return widget.NewLabelWithStyle(label, fyne.TextAlignTrailing, fyne.TextStyle{Bold: true})
}

type rowActionOptions struct {
	widgetType string
	copy       bool
	ellipsis   int
	export     string
}

func makeActionMenu(menuItems []*fyne.MenuItem, w fyne.Window) fyne.CanvasObject {
	d := fyne.CurrentApp().Driver()
	popUpMenu := widget.NewPopUpMenu(fyne.NewMenu("", menuItems...), w.Canvas())

	var button *widget.Button
	button = widget.NewButtonWithIcon("", theme.MoreVerticalIcon(), func() {
		buttonPos := d.AbsolutePositionForObject(button)
		buttonSize := button.Size()
		popUpMin := popUpMenu.MinSize()

		var popUpPos fyne.Position
		popUpPos.X = buttonPos.X + buttonSize.Width - popUpMin.Width
		popUpPos.Y = buttonPos.Y + buttonSize.Height
		popUpMenu.ShowAtPosition(popUpPos)
	})

	return button
}

func rowWithAction(label string, text string, opts rowActionOptions, w fyne.Window) []fyne.CanvasObject {

	actionMenu := []*fyne.MenuItem{}

	if opts.copy {
		action := &fyne.MenuItem{
			Label:  "Copy",
			Icon:   theme.ContentCopyIcon(),
			Action: copyAction(label, text, w),
		}
		actionMenu = append(actionMenu, action)
	}

	if opts.export != "" {
		action := &fyne.MenuItem{
			Label:  "Export",
			Icon:   icon.DownloadOutlinedIconThemed,
			Action: exportAction(opts.export, []byte(text), w),
		}
		actionMenu = append(actionMenu, action)
	}

	var v fyne.CanvasObject
	switch opts.widgetType {
	case "password":
		v = pawwidget.NewPasswordRevealer(text)
	case "url":
		u, err := url.Parse(text)
		if err == nil && strings.HasPrefix(u.Scheme, "http") {
			v = widget.NewHyperlink(text, u)
			break
		}
		v = &widget.Label{
			Text:      text,
			Alignment: fyne.TextAlignLeading,
			Wrapping:  fyne.TextWrapBreak,
		}
	default:
		labelTxt := strings.TrimRight(text, "\n")
		l := widget.NewLabel(labelTxt)
		s := container.NewScroll(l)
		newLines := strings.Count(text, "\n")
		if newLines > 0 {
			if newLines > 10 {
				newLines = 10
			}
			newSize := s.MinSize()
			newSize.Height = theme.TextSize() * float32(newLines)
			s.SetMinSize(newSize)
		}
		v = s
	}

	var o fyne.CanvasObject
	switch len(actionMenu) {
	case 0:
		o = widget.NewLabel("")
	case 1:
		e := actionMenu[0]
		o = widget.NewButtonWithIcon("", e.Icon, e.Action)
	default:
		o = makeActionMenu(actionMenu, w)
	}

	return []fyne.CanvasObject{
		labelWithStyle(label),
		container.NewBorder(nil, nil, nil, container.NewVBox(o), v),
	}
}

func exportAction(filename string, data []byte, w fyne.Window) func() {
	return func() {
		d := dialog.NewFileSave(func(uc fyne.URIWriteCloser, err error) {
			if uc == nil {
				// file open dialog has been cancelled
				return
			}
			defer uc.Close()
			uc.Write(data)
		}, w)
		d.SetFileName(filename)
		d.Show()
	}
}

func copyAction(label string, text string, w fyne.Window) func() {
	return func() {
		w.Clipboard().SetContent(text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: fmt.Sprintf("%s copied", label),
		})
	}
}
