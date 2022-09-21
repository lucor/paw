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

func NewFyneItem(item paw.Item) FyneItem {
	var fyneItem FyneItem
	switch item.GetMetadata().Type {
	case paw.NoteItemType:
		fyneItem = &Note{Note: item.(*paw.Note)}
	case paw.LoginItemType:
		fyneItem = &Login{Login: item.(*paw.Login)}
	case paw.PasswordItemType:
		fyneItem = &Password{Password: item.(*paw.Password)}
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
	copy     bool
	ellipsis int
	export   string
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

	l := labelWithStyle(label)
	t := makeLabelWithEllipis(text, opts.ellipsis)
	return []fyne.CanvasObject{l, container.NewBorder(nil, nil, nil, container.NewVBox(makeActionMenu(actionMenu, w)), t)}
}

func makeLabelWithEllipis(text string, ellipsis int) *widget.Label {
	labelText := text
	if ellipsis > 0 {
		labelText = text[0:ellipsis] + "..."
	}
	l := widget.NewLabel(labelText)
	l.Wrapping = fyne.TextWrapBreak
	return l
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

func copiableRow(label string, text string, w fyne.Window) []fyne.CanvasObject {
	t := widget.NewLabel(text)
	t.Wrapping = fyne.TextWrapBreak
	b := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), copyAction(label, text, w))

	l := labelWithStyle(label)
	return []fyne.CanvasObject{l, container.NewBorder(nil, nil, nil, container.NewVBox(b), t)}
}

func copiableLinkRow(label string, text string, w fyne.Window) []fyne.CanvasObject {
	var t fyne.CanvasObject
	t = widget.NewLabel(text)
	u, err := url.Parse(text)
	if err == nil && strings.HasPrefix(u.Scheme, "http") {
		t = widget.NewHyperlink(text, u)
	}

	b := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(text)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: fmt.Sprintf("%s copied", label),
		})
	})

	l := labelWithStyle(label)
	return []fyne.CanvasObject{l, container.NewBorder(nil, nil, nil, b, t)}
}

func copiablePasswordRow(label string, password string, w fyne.Window) []fyne.CanvasObject {
	passwordEntry := widget.NewPasswordEntry()
	passwordEntry.SetText(password)
	passwordEntry.Disable()
	passwordEntry.Validator = nil
	passwordCopyButton := widget.NewButtonWithIcon("Copy", theme.ContentCopyIcon(), func() {
		w.Clipboard().SetContent(password)
		fyne.CurrentApp().SendNotification(&fyne.Notification{
			Title:   "paw",
			Content: fmt.Sprintf("%s copied", label),
		})
	})
	l := labelWithStyle(label)
	return []fyne.CanvasObject{l, container.NewBorder(nil, nil, nil, passwordCopyButton, passwordEntry)}
}
