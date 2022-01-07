package paw

import "encoding/json"

type Imported struct {
	Items []Item
}

func (i *Imported) UnmarshalJSON(data []byte) error {
	if i == nil {
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
