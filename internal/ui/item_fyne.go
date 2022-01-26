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
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

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

func copiableRow(label string, text string, w fyne.Window) []fyne.CanvasObject {
	t := widget.NewLabel(text)
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
