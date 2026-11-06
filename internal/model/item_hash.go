package model

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
)

const itemHashExtensionName = "paw.pm/item-hash"

// ContentHash returns lowercase hexadecimal SHA-256 of deterministic item JSON,
// excluding history, modifiedAt, and the summary-only item hash extension.
func (item *Item) ContentHash() (string, error) {
	snapshot, err := item.Snapshot()
	if err != nil {
		return "", err
	}
	snapshot.ModifiedAt = nil
	return hashJSON(snapshot)
}

// StateHash returns lowercase hexadecimal SHA-256 of deterministic full-item JSON,
// including history and all timestamps, but excluding the summary-only item hash.
func (item *Item) StateHash() (string, error) {
	if err := item.Validate(); err != nil {
		return "", err
	}
	return hashJSON(item)
}

func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value, json.Deterministic(true))
	if err != nil {
		return "", fmt.Errorf("cannot encode item for hash: %w", err)
	}
	// Deterministic marshaling does not reorder object members inside raw CXF JSON values.
	// Reorder those members too so serialization order cannot look like an item edit.
	raw := jsontext.Value(data)
	if err := raw.Format(jsontext.ReorderRawObjects(true)); err != nil {
		return "", fmt.Errorf("cannot normalize item JSON for hash: %w", err)
	}
	hash := sha256.Sum256(raw)
	return hex.EncodeToString(hash[:]), nil
}

// PrepareItemEdit archives the persisted item only when content changed.
func PrepareItemEdit(previous, proposed *Item, capturedAt, modifiedAt uint64) (*Item, bool, error) {
	if previous == nil || proposed == nil || previous.ID != proposed.ID {
		return nil, false, fmt.Errorf("previous and proposed items must have the same ID")
	}
	oldHash, err := previous.ContentHash()
	if err != nil {
		return nil, false, fmt.Errorf("cannot hash persisted item: %w", err)
	}
	newHash, err := proposed.ContentHash()
	if err != nil {
		return nil, false, fmt.Errorf("cannot hash proposed item: %w", err)
	}
	data, err := json.Marshal(proposed, json.Deterministic(true))
	if err != nil {
		return nil, false, fmt.Errorf("cannot copy proposed item: %w", err)
	}
	var updated Item
	if err := json.Unmarshal(data, &updated); err != nil {
		return nil, false, fmt.Errorf("cannot copy proposed item: %w", err)
	}
	history, err := previous.History()
	if err != nil {
		return nil, false, fmt.Errorf("cannot read persisted history: %w", err)
	}
	if oldHash == newHash {
		if err := updated.SetHistory(history); err != nil {
			return nil, false, err
		}
		updated.ModifiedAt = previous.ModifiedAt
		return &updated, false, nil
	}
	if err := history.AddSnapshot(previous, capturedAt); err != nil {
		return nil, false, err
	}
	if err := updated.SetHistory(history); err != nil {
		return nil, false, err
	}
	updated.ModifiedAt = &modifiedAt
	return &updated, true, nil
}
