package paw

import (
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// ItemType represents the Item type
type ItemType int

const (
	// MetadataItemType is the Metadata Item type
	MetadataItemType ItemType = 1 << iota
	// NoteItemType is the Note Item type
	NoteItemType
	// PasswordItemType is the Password Item type
	PasswordItemType
	// WebsiteItemType is the Website Item type
	WebsiteItemType
)

func (it ItemType) String() string {
	switch it {
	case MetadataItemType:
		return "Metadata"
	case NoteItemType:
		return "Note"
	case PasswordItemType:
		return "Password"
	case WebsiteItemType:
		return "Website"
	}
	return "invalid"
}

// Item wraps all methods allow to generate a password with paw
type Item interface {
	// ID returns the identity ID
	ID() string

	// Type returns a widget label that represents the identity type
	Type() ItemType

	GetMetadata() *Metadata

	fmt.Stringer
}

// FyneObject wraps all methods allow to hanle an Item as Fyne object
type FyneObject interface {
	// Type returns a widget icon for the identity type
	Icon() *widget.Icon
	// Show returns a fyne CanvasObject used to view the identity
	Show(w fyne.Window) fyne.CanvasObject
	// Edit returns a fyne CanvasObject used to edit the identity
	Edit(w fyne.Window) (fyne.CanvasObject, Item)
	//
	InfoUI() fyne.CanvasObject
}

// Item represents the basic paw identity
type Metadata struct {
	// Note holds optional note
	Note string
	// Title reprents the item label. It is also used internally to generate the item's ID
	Title string
	// Modified holds the modification date
	Modified time.Time
	// Created holds the creation date
	Created   time.Time
	Revision  int    // Revision reprents the identity revision
	Revisions []Item // Revision holds the identity revisions

	OnLabelChanged func(string)
}

func (id *Metadata) ID() string {
	return fmt.Sprintf("metadata/%s", strings.ToLower(id.Title))
}

func (id *Metadata) GetMetadata() *Metadata {
	return id
}

func (id *Metadata) String() string {
	return id.Title
}

func (id *Metadata) Type() ItemType {
	return MetadataItemType
}

// ByID implements sort.Interface Metadata on the ID value.
type ByString []Item

func (ids ByString) Len() int { return len(ids) }
func (ids ByString) Less(i, j int) bool {
	return ids[i].String() < ids[j].String()
}
func (ids ByString) Swap(i, j int) { ids[i], ids[j] = ids[j], ids[i] }

func (id *Metadata) InfoUI() fyne.CanvasObject {
	return container.New(
		layout.NewFormLayout(),
		widget.NewLabel("Modified"),
		widget.NewLabel(id.Modified.Format(time.RFC1123)),
		widget.NewLabel("Created"),
		widget.NewLabel(id.Created.Format(time.RFC1123)),
	)
}

func titleRow(icon *widget.Icon, text string) []fyne.CanvasObject {
	t := canvas.NewText(text, theme.ForegroundColor())
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextHeadingSize()
	return []fyne.CanvasObject{
		icon,
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
