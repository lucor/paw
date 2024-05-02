// Copyright 2021 the Paw Authors. All rights reserved.
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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
