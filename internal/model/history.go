package model

import (
	"bytes"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"

	"go.lucor.dev/cxf"
)

const (
	historyExtensionName = "paw.pm/history"
	historySchemaVersion = 1
	maxHistoryEntries    = 10
)

// History stores previous versions of one item, oldest first.
type History struct {
	SchemaVersion uint8          `json:"schemaVersion"`
	Entries       []HistoryEntry `json:"entries"`
}

// HistoryEntry is a previous item and the UTC Unix time it was archived.
type HistoryEntry struct {
	CapturedAt uint64   `json:"capturedAt"`
	Item       cxf.Item `json:"item"`
}

type historyExtension struct {
	Name          string          `json:"name"`
	SchemaVersion *uint8          `json:"schemaVersion"`
	Entries       *[]HistoryEntry `json:"entries"`
}

// History returns the item's history, or an empty history when absent.
func (item *Item) History() (History, error) {
	if item == nil {
		return History{}, fmt.Errorf("item is nil")
	}
	index, raw, err := findHistoryExtension(item.Extensions)
	if err != nil {
		return History{}, err
	}
	if index < 0 {
		return History{
			SchemaVersion: historySchemaVersion,
			Entries:       []HistoryEntry{},
		}, nil
	}
	var extension historyExtension
	if err := json.Unmarshal(raw, &extension); err != nil || extension.Name != historyExtensionName || extension.SchemaVersion == nil || extension.Entries == nil {
		return History{}, fmt.Errorf("invalid item history extension")
	}
	history := History{
		SchemaVersion: *extension.SchemaVersion,
		Entries:       *extension.Entries,
	}
	if err := history.Validate(); err != nil {
		return History{}, err
	}
	for _, entry := range history.Entries {
		if entry.Item.ID != item.ID {
			return History{}, fmt.Errorf("history item ID does not match owner")
		}
	}
	return cloneHistory(history), nil
}

// SetHistory validates and replaces the item's history extension atomically.
func (item *Item) SetHistory(history History) error {
	if item == nil {
		return fmt.Errorf("item is nil")
	}
	if err := history.Validate(); err != nil {
		return err
	}
	for _, entry := range history.Entries {
		if entry.Item.ID != item.ID {
			return fmt.Errorf("history item ID does not match owner")
		}
	}
	raw, err := json.Marshal(historyExtension{
		Name:          historyExtensionName,
		SchemaVersion: &history.SchemaVersion,
		Entries:       &history.Entries,
	}, json.Deterministic(true))
	if err != nil {
		return fmt.Errorf("cannot encode item history")
	}
	extensions := make([]jsontext.Value, 0, len(item.Extensions)+1)
	for _, extension := range item.Extensions {
		var header struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(extension, &header); err != nil {
			return fmt.Errorf("cannot decode item extension name")
		}
		if header.Name != historyExtensionName {
			extensions = append(extensions, append(jsontext.Value(nil), extension...))
		}
	}
	item.Extensions = append(extensions, raw)
	return nil
}

// Snapshot returns a detached copy without its history extension.
func (item *Item) Snapshot() (*cxf.Item, error) {
	if err := item.Validate(); err != nil {
		return nil, err
	}
	data, err := json.Marshal(item, json.Deterministic(true))
	if err != nil {
		return nil, fmt.Errorf("cannot encode item snapshot: %w", err)
	}
	var snapshot cxf.Item
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return nil, fmt.Errorf("cannot decode item snapshot: %w", err)
	}
	extensions := snapshot.Extensions[:0]
	for _, extension := range snapshot.Extensions {
		var header struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(extension, &header); err != nil {
			return nil, fmt.Errorf("cannot decode snapshot extension name")
		}
		if header.Name != historyExtensionName && header.Name != itemHashExtensionName {
			extensions = append(extensions, extension)
		}
	}
	snapshot.Extensions = extensions
	return &snapshot, nil
}

// AddSnapshot appends a previous version and retains the newest ten entries.
func (history *History) AddSnapshot(previous *Item, capturedAt uint64) error {
	if history == nil || previous == nil {
		return fmt.Errorf("history and previous item are required")
	}
	if history.SchemaVersion == 0 && history.Entries == nil {
		history.SchemaVersion = historySchemaVersion
		history.Entries = []HistoryEntry{}
	}
	if err := history.Validate(); err != nil {
		return err
	}
	if len(history.Entries) != 0 && history.Entries[0].Item.ID != previous.ID {
		return fmt.Errorf("history item ID does not match previous item")
	}
	snapshot, err := previous.Snapshot()
	if err != nil {
		return err
	}
	entries := append(history.Entries, HistoryEntry{
		CapturedAt: capturedAt,
		Item:       *snapshot,
	})
	if len(entries) > maxHistoryEntries {
		entries = append([]HistoryEntry(nil), entries[len(entries)-maxHistoryEntries:]...)
	}
	history.SchemaVersion, history.Entries = historySchemaVersion, entries
	return nil
}

// PrepareItemRestore builds a restored item without changing the current item.
// The expected entry must match the selected entry returned by the displayed History.
// now is the UTC Unix timestamp used for capture and modification times.
func PrepareItemRestore(
	current *Item,
	index int,
	expected *HistoryEntry,
	now uint64,
) (*Item, bool, error) {
	if current == nil {
		return nil, false, fmt.Errorf("current item is required")
	}
	if err := current.Validate(); err != nil {
		return nil, false, fmt.Errorf("invalid current item: %w", err)
	}
	history, err := current.History()
	if err != nil {
		return nil, false, fmt.Errorf("cannot read current item history: %w", err)
	}
	if len(history.Entries) == 0 {
		return nil, false, fmt.Errorf("item has no history")
	}
	if index < 0 || index >= len(history.Entries) {
		return nil, false, fmt.Errorf("history entry index %d is out of range", index)
	}
	if expected == nil {
		return nil, false, fmt.Errorf("expected history entry is required")
	}
	selected := history.Entries[index]
	if selected.CapturedAt != expected.CapturedAt {
		return nil, false, fmt.Errorf("selected history entry is stale")
	}
	selectedJSON, err := json.Marshal(selected.Item, json.Deterministic(true))
	if err != nil {
		return nil, false, fmt.Errorf("cannot encode selected history entry: %w", err)
	}
	expectedJSON, err := json.Marshal(expected.Item, json.Deterministic(true))
	if err != nil {
		return nil, false, fmt.Errorf("cannot encode expected history entry: %w", err)
	}
	if !bytes.Equal(selectedJSON, expectedJSON) {
		return nil, false, fmt.Errorf("selected history entry is stale")
	}

	var restored Item
	if err := json.Unmarshal(selectedJSON, &restored); err != nil {
		return nil, false, fmt.Errorf("cannot decode historical item: %w", err)
	}
	restored.ID = current.ID
	if current.CreationAt == nil {
		restored.CreationAt = nil
	} else {
		creationAt := *current.CreationAt
		restored.CreationAt = &creationAt
	}

	currentHash, err := current.ContentHash()
	if err != nil {
		return nil, false, fmt.Errorf("cannot hash current item: %w", err)
	}
	restoredHash, err := restored.ContentHash()
	if err != nil {
		return nil, false, fmt.Errorf("cannot hash restored item: %w", err)
	}
	if currentHash == restoredHash {
		return current, false, nil
	}
	if err := history.AddSnapshot(current, now); err != nil {
		return nil, false, fmt.Errorf("cannot archive current item: %w", err)
	}
	if err := restored.SetHistory(history); err != nil {
		return nil, false, fmt.Errorf("cannot preserve item history: %w", err)
	}
	modifiedAt := now
	restored.ModifiedAt = &modifiedAt
	return &restored, true, nil
}

// Validate checks history version, ordering, item validity, and retention.
func (history History) Validate() error {
	if history.SchemaVersion != historySchemaVersion {
		return fmt.Errorf("unsupported history schema version %d", history.SchemaVersion)
	}
	if history.Entries == nil || len(history.Entries) > maxHistoryEntries {
		return fmt.Errorf("invalid history entries")
	}
	var id string
	for i, entry := range history.Entries {
		if i == 0 {
			id = entry.Item.ID
		} else if entry.Item.ID != id {
			return fmt.Errorf("history entries have inconsistent item IDs")
		}
		if err := entry.Item.Validate(); err != nil {
			return fmt.Errorf("invalid history snapshot: %w", err)
		}
		if _, _, err := findHistoryExtension(entry.Item.Extensions); err != nil {
			return err
		}
		if index, _, _ := findHistoryExtension(entry.Item.Extensions); index >= 0 {
			return fmt.Errorf("history snapshot contains history")
		}
		for _, extension := range entry.Item.Extensions {
			var header struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(extension, &header); err != nil {
				return fmt.Errorf("cannot decode history snapshot extension name")
			}
			if header.Name == itemHashExtensionName {
				return fmt.Errorf("history snapshot contains item hash")
			}
		}
		if _, err := ItemTypeOf(&entry.Item); err != nil {
			return fmt.Errorf("invalid history snapshot metadata: %w", err)
		}
	}
	return nil
}

func findHistoryExtension(extensions []jsontext.Value) (int, jsontext.Value, error) {
	index := -1
	for i, extension := range extensions {
		var header struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(extension, &header); err != nil {
			return -1, nil, fmt.Errorf("cannot decode item extension name")
		}
		if header.Name == historyExtensionName {
			if index >= 0 {
				return -1, nil, fmt.Errorf("duplicate item history extension")
			}
			index = i
		}
	}
	if index < 0 {
		return -1, nil, nil
	}
	return index, extensions[index], nil
}

func cloneHistory(history History) History {
	data, _ := json.Marshal(history, json.Deterministic(true))
	var clone History
	_ = json.Unmarshal(data, &clone)
	if clone.Entries == nil {
		clone.Entries = []HistoryEntry{}
	}
	return clone
}
