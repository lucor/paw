// SPDX-FileCopyrightText: 2026 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"encoding/json"
	"fmt"
)

// DecodeVault reads legacy vault JSON and attaches its runtime encryption key.
func DecodeVault(data []byte, key *Key) (*Vault, error) {
	var vault *Vault
	if err := json.Unmarshal(data, &vault); err != nil {
		// Decoder errors can contain saved values, so keep them out of reports.
		return nil, fmt.Errorf("could not decode legacy vault JSON")
	}
	if vault == nil {
		return nil, fmt.Errorf("legacy vault is null")
	}
	vault.key = key
	return vault, nil
}

// DecodeItem reads legacy item JSON using the type recorded in the vault.
func DecodeItem(data []byte, itemType ItemType) (Item, error) {
	var item Item
	switch itemType {
	case NoteItemType:
		item = &Note{}
	case PasswordItemType:
		item = &Password{}
	case LoginItemType:
		item = &Login{}
	case SSHKeyItemType:
		item = &SSHKey{}
	default:
		return nil, fmt.Errorf("unsupported legacy item type %d", itemType)
	}
	if err := json.Unmarshal(data, &item); err != nil {
		// Decoder errors can contain saved values, so keep them out of reports.
		return nil, fmt.Errorf("could not decode legacy item JSON")
	}
	if item == nil {
		return nil, fmt.Errorf("legacy item is null")
	}
	return item, nil
}
