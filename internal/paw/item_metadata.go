package paw

import (
	"encoding/hex"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/crypto/blake2b"
	"lucor.dev/paw/internal/icon"
)

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
	Favicon *icon.Favicon `json:"favicon,omitempty"`
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
	if m.Favicon != nil {
		return m.Favicon
	}
	switch m.Type {
	case NoteItemType:
		return icon.NoteOutlinedIconThemed
	case PasswordItemType:
		return icon.PasswordOutlinedIconThemed
	case LoginItemType:
		return icon.PublicOutlinedIconThemed
	}
	return icon.PawIcon
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
