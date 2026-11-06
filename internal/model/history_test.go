package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItemHistoryRoundTripAndSnapshot(t *testing.T) {
	item, err := NewNote("Example", "secret note")
	require.NoError(t, err)
	item.Metadata.Favicon = &Favicon{Host: "example.com", Data: []byte("icon"), Format: "png"}
	item.Tags = []string{"work"}
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/future","value":9007199254740993}`))

	history, err := item.History()
	require.NoError(t, err)
	require.Empty(t, history.Entries)
	require.NoError(t, history.AddSnapshot(item, 20))
	item.Title = "Changed"
	require.Len(t, history.Entries, 1)
	require.Equal(t, "Example", history.Entries[0].Item.Title)
	require.Equal(t, item.ID, history.Entries[0].Item.ID)
	require.NotContains(t, string(history.Entries[0].Item.Extensions[len(history.Entries[0].Item.Extensions)-1]), historyExtensionName)

	require.NoError(t, item.SetHistory(history))
	flatSnapshot, err := item.Snapshot()
	require.NoError(t, err)
	require.Len(t, flatSnapshot.Extensions, 2)
	for _, extension := range flatSnapshot.Extensions {
		require.NotContains(t, string(extension), historyExtensionName)
	}
	encoded, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	var decoded Item
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	got, err := decoded.History()
	require.NoError(t, err)
	require.Equal(t, history, got)
	require.Contains(t, string(encoded), "example.com/future")
	require.Contains(t, string(encoded), "secret note")

	summary, err := item.Summary()
	require.NoError(t, err)
	require.Len(t, summary.Extensions, 1)
	require.Contains(t, string(summary.Extensions[0]), "favicon")
	require.NotContains(t, string(summary.Extensions[0]), historyExtensionName)
	require.NotContains(t, string(summary.Extensions[0]), "secret note")
}

func TestItemHistoryAppendsInInsertionOrderAndRoundTrips(t *testing.T) {
	item, err := NewNote("Example", "secret")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 20))
	item.Title = "Second version"
	require.NoError(t, history.AddSnapshot(item, 10))
	require.Len(t, history.Entries, 2)
	require.Equal(t, "Example", history.Entries[0].Item.Title)
	require.Equal(t, "Second version", history.Entries[1].Item.Title)
	require.Equal(t, uint64(20), history.Entries[0].CapturedAt)
	require.Equal(t, uint64(10), history.Entries[1].CapturedAt)
	require.NoError(t, history.Validate(), "insertion order is independent of capture timestamps")
	require.NoError(t, item.SetHistory(history))
	data, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	var decoded Item
	require.NoError(t, json.Unmarshal(data, &decoded))
	got, err := decoded.History()
	require.NoError(t, err)
	require.Equal(t, history, got)
}

func TestItemHistoryRetainsLatestTenInInsertionOrder(t *testing.T) {
	item, err := NewNote("Example", "secret")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	for version := 1; version <= maxHistoryEntries; version++ {
		item.Title = fmt.Sprintf("Version %02d", version)
		require.NoError(t, history.AddSnapshot(item, uint64(100-version)))
	}
	require.Len(t, history.Entries, maxHistoryEntries)
	require.Equal(t, "Version 01", history.Entries[0].Item.Title)
	require.Equal(t, "Version 10", history.Entries[maxHistoryEntries-1].Item.Title)
	require.Equal(t, uint64(90), history.Entries[maxHistoryEntries-1].CapturedAt)
	for version := maxHistoryEntries + 1; version <= 15; version++ {
		item.Title = fmt.Sprintf("Version %02d", version)
		require.NoError(t, history.AddSnapshot(item, uint64(100-version)))
		if version == maxHistoryEntries+1 {
			require.Len(t, history.Entries, maxHistoryEntries)
			require.Equal(t, "Version 02", history.Entries[0].Item.Title)
			require.Equal(t, "Version 11", history.Entries[maxHistoryEntries-1].Item.Title)
		}
	}
	require.Len(t, history.Entries, maxHistoryEntries)
	require.Equal(t, "Version 06", history.Entries[0].Item.Title)
	require.Equal(t, "Version 15", history.Entries[maxHistoryEntries-1].Item.Title)
	require.Equal(t, uint64(85), history.Entries[maxHistoryEntries-1].CapturedAt)
}

func TestItemHistoryRejectsInvalidData(t *testing.T) {
	for _, test := range []struct {
		name      string
		extension string
	}{
		{name: "malformed", extension: `{"name":"paw.pm/history","schemaVersion":1,"entries":"bad"}`},
		{name: "unsupported version", extension: `{"name":"paw.pm/history","schemaVersion":2,"entries":[]}`},
		{name: "duplicate", extension: `{"name":"paw.pm/history","schemaVersion":1,"entries":[]}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			item, err := NewNote("Example", "secret")
			require.NoError(t, err)
			item.Extensions = append(item.Extensions, jsontext.Value(test.extension))
			if test.name == "duplicate" {
				item.Extensions = append(item.Extensions, jsontext.Value(test.extension))
			}
			_, err = item.History()
			require.Error(t, err)
			_, err = json.Marshal(item, json.Deterministic(true))
			require.Error(t, err)
		})
	}
}

func TestItemHistorySnapshotPreservesCredentialsAndUnknownExtensions(t *testing.T) {
	item, err := NewNote("Example", "secret")
	require.NoError(t, err)
	item.Credentials = append(item.Credentials, jsontext.Value(`{"type":"future","value":"private"}`))
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/future","value":9007199254740993}`))
	snapshot, err := item.Snapshot()
	require.NoError(t, err)
	require.Equal(t, item.Credentials, snapshot.Credentials)
	require.Len(t, snapshot.Extensions, 2)
	require.JSONEq(t, `{"name":"example.com/future","value":9007199254740993}`, string(snapshot.Extensions[1]))
	item.Credentials[0][0] = 'x'
	require.NotEqual(t, item.Credentials[0], snapshot.Credentials[0])
	for _, extension := range snapshot.Extensions {
		var header struct {
			Name string `json:"name"`
		}
		require.NoError(t, json.Unmarshal(extension, &header))
		require.NotEqual(t, historyExtensionName, header.Name)
	}
}

func TestItemHistoryRejectsItemHashInSnapshot(t *testing.T) {
	item, err := NewNote("Example", "secret")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 10))
	history.Entries[0].Item.Extensions = append(history.Entries[0].Item.Extensions, jsontext.Value(`{"name":"paw.pm/item-hash","algorithm":"sha256","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	require.ErrorContains(t, history.Validate(), "history snapshot contains item hash")
}
