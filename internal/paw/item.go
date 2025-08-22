// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later


package paw

import (
	"fmt"
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
	// LoginItemType is the Website Item type
	LoginItemType
	// SSHKeyItemType is the SSH Key Item type
	SSHKeyItemType
)

// Label returns the item type label used in the UI
func (it ItemType) Label() string {
	switch it {
	case MetadataItemType:
		return "Metadata"
	case NoteItemType:
		return "Note"
	case PasswordItemType:
		return "Password"
	case LoginItemType:
		return "Login"
	case SSHKeyItemType:
		return "SSH key"
	}
	return "Invalid item type"
}

// String returns the item type string representation
func (it ItemType) String() string {
	switch it {
	case MetadataItemType:
		return "metadata"
	case NoteItemType:
		return "note"
	case PasswordItemType:
		return "password"
	case LoginItemType:
		return "login"
	case SSHKeyItemType:
		return "ssh_key"
	}
	return "invalid"
}

// ItemTypeFromString returns the item type from a string
func ItemTypeFromString(v string) (ItemType, error) {
	var itemType ItemType
	var err error
	switch v {
	case LoginItemType.String():
		itemType = LoginItemType
	case NoteItemType.String():
		itemType = NoteItemType
	case PasswordItemType.String():
		itemType = PasswordItemType
	case SSHKeyItemType.String():
		itemType = SSHKeyItemType
	default:
		err = fmt.Errorf("invalid item type %q", v)
	}
	return itemType, err
}

// Item wraps all methods allow to generate a password with paw
type Item interface {
	// ID returns the identity ID
	ID() string

	GetMetadata() *Metadata

	fmt.Stringer
}

func NewItem(name string, itemType ItemType) (Item, error) {
	var item Item
	switch itemType {
	case LoginItemType:
		item = NewLogin()
	case NoteItemType:
		item = NewNote()
	case PasswordItemType:
		item = NewPassword()
	case SSHKeyItemType:
		item = NewSSHKey()
	default:
		return nil, fmt.Errorf("invalid item type %q", itemType)
	}
	item.GetMetadata().Name = name
	return item, nil
}
