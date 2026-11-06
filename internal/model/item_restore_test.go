package model

import (
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/paw"
)

func TestPrepareItemRestoreRestoresCompleteItemTypes(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T) (*Item, error)
	}{
		{
			name: "login",
			build: func(t *testing.T) (*Item, error) {
				return NewLogin("Login", "synthetic-user", "synthetic-password", PasswordSettings{Mode: paw.CustomPassword})
			},
		},
		{
			name: "password",
			build: func(t *testing.T) (*Item, error) {
				return NewPassword("Password", "synthetic-password", PasswordSettings{Mode: paw.CustomPassword})
			},
		},
		{
			name: "secure note",
			build: func(t *testing.T) (*Item, error) {
				return NewNote("Note", "synthetic note")
			},
		},
		{
			name: "ssh key",
			build: func(t *testing.T) (*Item, error) {
				return NewSSHKey("SSH key", validTestSSHKey(t), "synthetic passphrase", PasswordSettings{Mode: paw.CustomPassword})
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			original, err := test.build(t)
			require.NoError(t, err)
			creationAt, modifiedAt := uint64(10), uint64(20)
			original.CreationAt, original.ModifiedAt = &creationAt, &modifiedAt
			original.Metadata.Favicon = &Favicon{
				Host:   "example.com",
				Data:   []byte("synthetic-icon"),
				Format: "png",
			}
			original.Tags = []string{"synthetic-tag"}
			original.Scope = &cxf.CredentialScope{
				URLs:        []string{"https://example.com"},
				AndroidApps: []cxf.AndroidAppID{},
			}
			original.Extensions = append(original.Extensions, jsontext.Value(`{"name":"example.com/future","value":"preserved"}`))
			originalCredentials := append([]jsontext.Value(nil), original.Credentials...)

			history, err := original.History()
			require.NoError(t, err)
			require.NoError(t, history.AddSnapshot(original, 30))
			require.NoError(t, original.SetHistory(history))
			expected := history.Entries[0]

			current := *original
			current.Title = "Changed current"
			current.Credentials = append([]jsontext.Value(nil), current.Credentials...)
			current.Credentials[0] = jsontext.Value(`{"type":"future","value":"changed synthetic credential"}`)
			current.Metadata.Favicon = &Favicon{
				Host:   "changed.example",
				Data:   []byte("changed-icon"),
				Format: "svg",
			}
			current.Extensions = append([]jsontext.Value(nil), current.Extensions...)
			current.Extensions = append(current.Extensions, jsontext.Value(`{"name":"example.com/current-only","value":"discarded"}`))
			currentModifiedAt := uint64(40)
			current.ModifiedAt = &currentModifiedAt
			currentBefore := current.Title

			restored, changed, err := PrepareItemRestore(&current, 0, &expected, 50)
			require.NoError(t, err)
			require.True(t, changed)
			require.Equal(t, currentBefore, current.Title, "restore must not mutate the source item")
			require.Equal(t, original.ID, restored.ID)
			require.Equal(t, creationAt, *restored.CreationAt)
			require.Equal(t, uint64(50), *restored.ModifiedAt)
			require.Equal(t, original.Title, restored.Title)
			require.Equal(t, originalCredentials, restored.Credentials)
			require.Equal(t, original.Metadata, restored.Metadata)
			require.Equal(t, original.Tags, restored.Tags)
			require.Equal(t, original.Scope, restored.Scope)
			require.Contains(t, restored.Extensions, jsontext.Value(`{"name":"example.com/future","value":"preserved"}`))
			for _, extension := range restored.Extensions {
				var header struct {
					Name string `json:"name"`
				}
				require.NoError(t, json.Unmarshal(extension, &header))
				if header.Name != historyExtensionName {
					require.NotContains(t, string(extension), "current-only")
				}
			}
			require.Equal(t, expected, historyEntry(t, restored, 0))
			restoredHistory, err := restored.History()
			require.NoError(t, err)
			require.Len(t, restoredHistory.Entries, 2)
			require.Equal(t, "Changed current", restoredHistory.Entries[1].Item.Title)
		})
	}
}

func TestPrepareItemRestoreIsReversible(t *testing.T) {
	current, err := NewNote("D", "synthetic D")
	require.NoError(t, err)
	history, err := current.History()
	require.NoError(t, err)
	versions := []struct {
		title string
		value string
	}{
		{
			title: "A",
			value: "synthetic A",
		},
		{
			title: "B",
			value: "synthetic B",
		},
		{
			title: "C",
			value: "synthetic C",
		},
	}
	for i, version := range versions {
		current.Title = version.title
		current.Credentials[0] = jsontext.Value(`{"type":"note","content":{"fieldType":"string","value":"` + version.value + `"}}`)
		require.NoError(t, history.AddSnapshot(current, uint64(i+1)))
	}
	current.Title = "D"
	current.Credentials[0] = jsontext.Value(`{"type":"note","content":{"fieldType":"string","value":"synthetic D"}}`)
	require.NoError(t, current.SetHistory(history))

	firstSelection := historyEntry(t, current, 1)
	restored, changed, err := PrepareItemRestore(current, 1, &firstSelection, 100)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "B", restored.Title)

	restoredHistory, err := restored.History()
	require.NoError(t, err)
	currentSelection := restoredHistory.Entries[len(restoredHistory.Entries)-1]
	reverted, changed, err := PrepareItemRestore(restored, len(restoredHistory.Entries)-1, &currentSelection, 101)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "D", reverted.Title)
	revertedHistory, err := reverted.History()
	require.NoError(t, err)
	require.Equal(t, "B", revertedHistory.Entries[len(revertedHistory.Entries)-1].Item.Title)
}

func TestPrepareItemRestoreRetentionKeepsSelectedVersion(t *testing.T) {
	item, err := NewNote("Current", "synthetic current")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	for version := 0; version < maxHistoryEntries; version++ {
		item.Title = fmt.Sprintf("Version %02d", version)
		require.NoError(t, history.AddSnapshot(item, uint64(version+1)))
	}
	item.Title = "Current"
	require.NoError(t, item.SetHistory(history))
	selected := history.Entries[5]

	restored, changed, err := PrepareItemRestore(item, 5, &selected, 50)
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, "Version 05", restored.Title)
	restoredHistory, err := restored.History()
	require.NoError(t, err)
	require.Len(t, restoredHistory.Entries, maxHistoryEntries)
	require.Equal(t, "Version 01", restoredHistory.Entries[0].Item.Title)
	require.Equal(t, selected, restoredHistory.Entries[4])
	require.Equal(t, "Current", restoredHistory.Entries[maxHistoryEntries-1].Item.Title)
}

func TestPrepareItemRestoreNoOpAndSelectionErrors(t *testing.T) {
	item, err := NewNote("Same", "synthetic value")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 10))
	require.NoError(t, item.SetHistory(history))
	entry := history.Entries[0]
	modifiedAt := uint64(20)
	item.ModifiedAt = &modifiedAt
	unchanged, changed, err := PrepareItemRestore(item, 0, &entry, 30)
	require.NoError(t, err)
	require.False(t, changed)
	require.Same(t, item, unchanged)
	require.Equal(t, modifiedAt, *item.ModifiedAt)
	unchangedHistory, err := unchanged.History()
	require.NoError(t, err)
	require.Equal(t, history, unchangedHistory)

	for _, test := range []struct {
		name    string
		index   int
		expect  *HistoryEntry
		wantErr string
	}{
		{
			name:    "negative index",
			index:   -1,
			expect:  &entry,
			wantErr: "out of range",
		},
		{
			name:    "past end",
			index:   1,
			expect:  &entry,
			wantErr: "out of range",
		},
		{
			name:  "stale timestamp",
			index: 0,
			expect: &HistoryEntry{
				CapturedAt: 11,
				Item:       entry.Item,
			},
			wantErr: "stale",
		},
		{
			name:    "missing expectation",
			index:   0,
			wantErr: "expected history entry",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := PrepareItemRestore(item, test.index, test.expect, 30)
			require.ErrorContains(t, err, test.wantErr)
		})
	}

	empty, err := NewNote("Empty", "synthetic")
	require.NoError(t, err)
	_, _, err = PrepareItemRestore(empty, 0, &entry, 30)
	require.ErrorContains(t, err, "no history")
}

func TestPrepareItemRestoreRejectsMalformedAndUnsupportedHistory(t *testing.T) {
	for _, extension := range []string{
		`{"name":"paw.pm/history","schemaVersion":1,"entries":"invalid"}`,
		`{"name":"paw.pm/history","schemaVersion":2,"entries":[]}`,
	} {
		item, err := NewNote("Current", "synthetic")
		require.NoError(t, err)
		item.Extensions = append(item.Extensions, jsontext.Value(extension))
		_, _, err = PrepareItemRestore(item, 0, &HistoryEntry{}, 50)
		require.Error(t, err)
	}

	item, err := NewNote("Current", "synthetic")
	require.NoError(t, err)
	snapshot, err := item.Snapshot()
	require.NoError(t, err)
	snapshot.Extensions[0] = jsontext.Value(`{"name":"paw.pm/item","type":99}`)
	version := uint8(historySchemaVersion)
	entries := []HistoryEntry{
		{
			CapturedAt: 10,
			Item:       *snapshot,
		},
	}
	raw, err := json.Marshal(historyExtension{
		Name:          historyExtensionName,
		SchemaVersion: &version,
		Entries:       &entries,
	}, json.Deterministic(true))
	require.NoError(t, err)
	item.Extensions = append(item.Extensions, raw)
	_, _, err = PrepareItemRestore(item, 0, &entries[0], 50)
	require.ErrorContains(t, err, "invalid history snapshot metadata")
}

func historyEntry(t *testing.T, item *Item, index int) HistoryEntry {
	t.Helper()
	history, err := item.History()
	require.NoError(t, err)
	require.GreaterOrEqual(t, index, 0)
	require.Less(t, index, len(history.Entries))
	return history.Entries[index]
}
