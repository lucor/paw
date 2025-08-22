// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package paw

import (
	"encoding/json"
	"fmt"
)

type Imported struct {
	Items []Item
}

func (i *Imported) UnmarshalJSON(data []byte) error {
	if i.Items == nil {
		i.Items = make([]Item, 0)
	}
	v := map[string][]json.RawMessage{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	for itemType, messages := range v {
		var t Item
		for _, message := range messages {
			switch itemType {
			case NoteItemType.String():
				t = NewNote()
			case PasswordItemType.String():
				t = NewPassword()
			case LoginItemType.String():
				t = NewLogin()
			case SSHKeyItemType.String():
				t = NewSSHKey()
			default:
				return fmt.Errorf("unknown item type: %s", itemType)
			}

			err := json.Unmarshal(message, t)
			if err != nil {
				return err
			}
			i.Items = append(i.Items, t)
		}

	}
	return nil
}
