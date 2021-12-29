package paw

import (
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/blake2b"

	"lucor.dev/paw/internal/icon"
)

func init() {
	gob.Register((*Metadata)(nil))
	gob.Register((*icon.ThemedResource)(nil))
	gob.Register((*fyne.StaticResource)(nil))
}

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
		return "metadata"
	case NoteItemType:
		return "note"
	case PasswordItemType:
		return "password"
	case WebsiteItemType:
		return "website"
	}
	return "invalid"
}

// Item wraps all methods allow to generate a password with paw
type Item interface {
	// ID returns the identity ID
	ID() string

	GetMetadata() *Metadata

	fmt.Stringer
}

// FyneObject wraps all methods allow to handle an Item as Fyne object
type FyneObject interface {
	// Type returns a widget icon for the identity type
	Icon() fyne.Resource
	// Show returns a fyne CanvasObject used to view the identity
	Show(ctx context.Context, w fyne.Window) fyne.CanvasObject
	// Edit returns a fyne CanvasObject used to edit the identity
	Edit(ctx context.Context, w fyne.Window) (fyne.CanvasObject, Item)
	//
	InfoUI() fyne.CanvasObject
}

// FynePasswordGenerator wraps all methods to show a Fyne dialog to generate passwords
type FynePasswordGenerator interface {
	ShowPasswordGenerator(bind binding.String, password *Password, w fyne.Window)
}

// Item represents the basic paw identity
type Metadata struct {
	// Title reprents the item name
	Name string `json:"name,omitempty"`
	// Type represents the item type
	Type ItemType `json:"type,omitempty"`
	// Modified holds the modification date
	Modified time.Time `json:"modified,omitempty"`
	// Created holds the creation date
	Created time.Time `json:"created,omitempty"`
	// Icon
	IconResource fyne.Resource `json:"icon_resource,omitempty"`
}

func (m *Metadata) ID() string {
	key := append([]byte(m.Type.String()), []byte(m.Name)...)
	hash, err := blake2b.New256(key)
	if err != nil {
		panic(err)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func (m *Metadata) GetMetadata() *Metadata {
	return m
}

func (m *Metadata) String() string {
	return m.Name
}

func (m *Metadata) Icon() fyne.Resource {
	if m.IconResource != nil {
		return m.IconResource
	}
	switch m.Type {
	case NoteItemType:
		return icon.NoteOutlinedIconThemed
	case PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case WebsiteItemType:
		return icon.PublicOutlinedIconThemed
	}
	return nil
}

func (m *Metadata) InfoUI() fyne.CanvasObject {
	return container.New(
		layout.NewFormLayout(),
		widget.NewLabel("Modified"),
		widget.NewLabel(m.Modified.Format(time.RFC1123)),
		widget.NewLabel("Created"),
		widget.NewLabel(m.Created.Format(time.RFC1123)),
	)
}

// ByID implements sort.Interface Metadata on the ID value.
type ByString []*Metadata

func (s ByString) Len() int { return len(s) }
func (s ByString) Less(i, j int) bool {
	return s[i].String() < s[j].String()
}
func (s ByString) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

func titleRow(icon fyne.Resource, text string) []fyne.CanvasObject {
	t := canvas.NewText(text, theme.ForegroundColor())
	t.TextStyle = fyne.TextStyle{Bold: true}
	t.TextSize = theme.TextHeadingSize()
	return []fyne.CanvasObject{
		widget.NewIcon(icon),
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
