package model

import (
	"encoding/json"
	"fmt"

	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

const itemExtensionName = "paw.pm/item"

// ItemTypeOf reads Paw's item category from a CXF summary extension.
func ItemTypeOf(item *cxf.Item) (paw.ItemType, error) {
	if item == nil {
		return 0, fmt.Errorf("item summary is nil")
	}
	index, _, err := findItemExtension(item.Extensions)
	if err != nil {
		return 0, err
	}
	if index < 0 {
		return 0, fmt.Errorf("missing Paw item metadata")
	}
	metadata, err := decodeItemExtension(item.Extensions[index])
	if err != nil {
		return 0, err
	}
	return metadata.Type, nil
}

// Item combines a CXF item with Paw-specific metadata.
// Pass *Item to encoding/json, including items held in maps and other containers.
type Item struct {
	cxf.Item
	// Metadata contains Paw item data stored in its extension.
	Metadata ItemMetadata `json:"-"`
}

// ItemMetadata holds fields Paw stores in the item extension.
type ItemMetadata struct {
	// Type identifies the item's Paw category.
	Type paw.ItemType
	// Favicon is the optional icon shown for the item.
	Favicon *paw.Favicon
	// AddToAgent records whether the SSH key should be loaded into the agent.
	AddToAgent bool
}

// Summary returns the CXF item stored in the vault account for this item.
func (item *Item) Summary() (*cxf.Item, error) {
	if err := item.Validate(); err != nil {
		return nil, err
	}
	extension, err := encodeItemExtension(nil, item.Metadata)
	if err != nil {
		return nil, fmt.Errorf("cannot encode item summary metadata: %w", err)
	}

	var creationAt, modifiedAt *uint64
	if item.CreationAt != nil {
		value := *item.CreationAt
		creationAt = &value
	}
	if item.ModifiedAt != nil {
		value := *item.ModifiedAt
		modifiedAt = &value
	}

	var subtitle *string
	if item.Subtitle != nil {
		value := *item.Subtitle
		subtitle = &value
	}

	var scope *cxf.CredentialScope
	if item.Scope != nil {
		value := *item.Scope
		value.URLs = append([]string{}, item.Scope.URLs...)
		value.AndroidApps = append([]cxf.AndroidAppID{}, item.Scope.AndroidApps...)
		scope = &value
	}

	return &cxf.Item{
		ID:          item.ID,
		CreationAt:  creationAt,
		ModifiedAt:  modifiedAt,
		Title:       item.Title,
		Subtitle:    subtitle,
		Scope:       scope,
		Credentials: []json.RawMessage{},
		Extensions:  []json.RawMessage{extension},
	}, nil
}

// Validate checks the CXF item and Paw metadata.
func (item *Item) Validate() error {
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	if err := item.Item.Validate(); err != nil {
		return fmt.Errorf("invalid CXF item: %w", err)
	}
	if !validItemType(item.Metadata.Type) {
		return fmt.Errorf("invalid Paw item type %d", item.Metadata.Type)
	}
	if _, _, err := findItemExtension(item.Extensions); err != nil {
		return err
	}
	return nil
}

// MarshalJSON encodes CXF fields and Paw metadata in the item extension.
func (item *Item) MarshalJSON() ([]byte, error) {
	if err := item.Validate(); err != nil {
		return nil, err
	}
	extensions := append([]json.RawMessage(nil), item.Extensions...)
	index, raw, err := findItemExtension(extensions)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeItemExtension(raw, item.Metadata)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		extensions = append(extensions, encoded)
	} else {
		extensions[index] = encoded
	}

	document := item.Item
	document.Extensions = extensions
	return json.Marshal(document)
}

// UnmarshalJSON decodes a CXF item and its Paw metadata atomically.
func (item *Item) UnmarshalJSON(data []byte) error {
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	var document cxf.Item
	if err := json.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("cannot decode CXF item: %w", err)
	}
	metadata, _, err := findItemExtension(document.Extensions)
	if err != nil {
		return err
	}
	if metadata < 0 {
		return fmt.Errorf("missing Paw item metadata")
	}
	decoded, err := decodeItemExtension(document.Extensions[metadata])
	if err != nil {
		return err
	}
	*item = Item{Item: document, Metadata: decoded}
	return nil
}

// findItemExtension returns the Paw metadata extension and its position.
func findItemExtension(extensions []json.RawMessage) (int, json.RawMessage, error) {
	index := -1
	var raw json.RawMessage
	for i, extension := range extensions {
		var header struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(extension, &header); err != nil {
			return -1, nil, fmt.Errorf("cannot decode item extension name")
		}
		if header.Name != itemExtensionName {
			continue
		}
		if index >= 0 {
			return -1, nil, fmt.Errorf("duplicate Paw item metadata")
		}
		index = i
		raw = extension
	}
	return index, raw, nil
}

// decodeItemExtension reads the Paw fields from its extension.
func decodeItemExtension(raw json.RawMessage) (ItemMetadata, error) {
	var extension struct {
		Name       string        `json:"name"`
		Type       *paw.ItemType `json:"type"`
		Favicon    *paw.Favicon  `json:"favicon"`
		AddToAgent bool          `json:"addToAgent"`
	}
	if err := json.Unmarshal(raw, &extension); err != nil {
		return ItemMetadata{}, fmt.Errorf("cannot decode Paw item metadata")
	}
	if extension.Name != itemExtensionName || extension.Type == nil || !validItemType(*extension.Type) {
		return ItemMetadata{}, fmt.Errorf("invalid Paw item metadata")
	}
	return ItemMetadata{Type: *extension.Type, Favicon: extension.Favicon, AddToAgent: extension.AddToAgent}, nil
}

// encodeItemExtension updates known Paw fields and preserves unknown members.
func encodeItemExtension(raw json.RawMessage, metadata ItemMetadata) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, fmt.Errorf("cannot decode Paw item metadata")
		}
	}
	name, err := json.Marshal(itemExtensionName)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw item metadata")
	}
	typeValue, err := json.Marshal(metadata.Type)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw item metadata")
	}
	fields["name"] = name
	fields["type"] = typeValue
	if metadata.AddToAgent {
		fields["addToAgent"], err = json.Marshal(true)
		if err != nil {
			return nil, fmt.Errorf("cannot encode Paw item metadata")
		}
	} else {
		delete(fields, "addToAgent")
	}
	if metadata.Favicon == nil {
		delete(fields, "favicon")
	} else {
		favicon, err := encodeItemFavicon(fields["favicon"], metadata.Favicon)
		if err != nil {
			return nil, fmt.Errorf("cannot encode Paw item favicon")
		}
		fields["favicon"] = favicon
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("cannot encode Paw item metadata")
	}
	return encoded, nil
}

// encodeItemFavicon preserves unknown icon fields while updating Paw's fields.
func encodeItemFavicon(raw json.RawMessage, favicon *paw.Favicon) (json.RawMessage, error) {
	fields := make(map[string]json.RawMessage)
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &fields); err != nil {
			return nil, err
		}
	}
	if fields == nil {
		fields = make(map[string]json.RawMessage)
	}
	encoded, err := json.Marshal(favicon)
	if err != nil {
		return nil, err
	}
	var known map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &known); err != nil {
		return nil, err
	}
	// Remove omitted known fields so clearing a field also clears its stored value.
	for _, name := range []string{"host", "data", "format"} {
		delete(fields, name)
	}
	for name, value := range known {
		fields[name] = value
	}
	return json.Marshal(fields)
}

// validItemType reports whether itemType is a supported Paw item category.
func validItemType(itemType paw.ItemType) bool {
	switch itemType {
	case paw.NoteItemType, paw.PasswordItemType, paw.LoginItemType, paw.SSHKeyItemType:
		return true
	default:
		return false
	}
}
