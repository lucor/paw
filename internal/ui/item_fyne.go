// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package ui

import (
	"context"
	"encoding/json"
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

// FyneItemWidget wraps all methods allow to handle a paw.Item as Fyne Widget
type FyneItemWidget interface {
	// Icon returns a fyne resource associated to the item
	Icon() fyne.Resource
	// Show returns a fyne CanvasObject used to view the item
	Show(ctx context.Context, w fyne.Window) fyne.CanvasObject
	// Edit returns a fyne CanvasObject used to edit the item
	Edit(ctx context.Context, key *paw.Key, w fyne.Window) fyne.CanvasObject
	// Item returns a deep copy of the embedded paw item
	// It will panic if the copy fails
	Item() paw.Item
	// OnSubmit performs the necessary actions to update the item with the latest data
	// and returns a deep copy of the embedded paw item
	OnSubmit() (paw.Item, error)
}

// FynePasswordGenerator wraps all methods to show a Fyne dialog to generate passwords
type FynePasswordGenerator interface {
	ShowPasswordGenerator(bind binding.String, password *paw.Password, w fyne.Window)
}

func NewFyneItemWidget(item paw.Item, preferences *paw.Preferences) FyneItemWidget {
	switch item.GetMetadata().Type {
	case paw.NoteItemType:
		return NewNoteWidget(item.(*paw.Note))
	case paw.LoginItemType:
		return NewLoginWidget(item.(*paw.Login), preferences)
	case paw.PasswordItemType:
		return NewPasswordWidget(item.(*paw.Password), preferences)
	case paw.SSHKeyItemType:
		return NewSSHWidget(item.(*paw.SSHKey), preferences)
	}
	panic(fmt.Sprintf("unsupported item type %q", item.GetMetadata().Type))
}

func titleRow(icon fyne.Resource, text string) []fyne.CanvasObject {
	t := canvas.NewText(text, theme.Color(theme.ColorNameForeground))
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextHeadingSize()
	i := canvas.NewImageFromResource(icon)
	i.FillMode = canvas.ImageFillContain
	i.SetMinSize(fyne.NewSize(32, 32))
	return []fyne.CanvasObject{
		container.NewCenter(i),
		t,
	}
}

func deepCopyItem(src, dst paw.Item) error {
	bytes, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(bytes, dst)
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
		rows := strings.Count(text, "\n") + 1
		if rows > 0 {
			if rows > 10 {
				rows = 10
			}
			newSize := s.MinSize()
			newSize.Height = (theme.TextSize()+theme.InnerPadding())*float32(rows) + theme.InnerPadding()*2
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
		fyne.CurrentApp().Clipboard().SetContent(text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: fmt.Sprintf("%s copied", label),
		})
	}
}

func newValidatioError(msg string) error {
	return &validationError{
		msg: msg,
	}
}

type validationError struct {
	msg string
}

func (e *validationError) Error() string {
	return e.msg
}

func requiredValidator(msg string) fyne.StringValidator {
	return func(text string) error {
		if text == "" {
			return newValidatioError(msg)
		}
		return nil
	}
}
