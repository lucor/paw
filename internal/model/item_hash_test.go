package model

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestItemHashesAreDeterministicAndSeparateContentFromState(t *testing.T) {
	item, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/future","opaque":{"n":9007199254740993}}`))
	content, err := item.ContentHash()
	require.NoError(t, err)
	state, err := item.StateHash()
	require.NoError(t, err)
	for range 5 {
		got, err := item.ContentHash()
		require.NoError(t, err)
		require.Equal(t, content, got)
		got, err = item.StateHash()
		require.NoError(t, err)
		require.Equal(t, state, got)
	}
	modified := *item.ModifiedAt + 10
	item.ModifiedAt = &modified
	contentAfterTimestamp, err := item.ContentHash()
	require.NoError(t, err)
	require.Equal(t, content, contentAfterTimestamp)
	stateAfterTimestamp, err := item.StateHash()
	require.NoError(t, err)
	require.NotEqual(t, state, stateAfterTimestamp)
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 20))
	require.NoError(t, item.SetHistory(history))
	contentAfterHistory, err := item.ContentHash()
	require.NoError(t, err)
	require.Equal(t, content, contentAfterHistory)
	stateAfterHistory, err := item.StateHash()
	require.NoError(t, err)
	require.NotEqual(t, stateAfterTimestamp, stateAfterHistory)
	data, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	var decoded Item
	require.NoError(t, json.Unmarshal(data, &decoded))
	decodedContent, err := decoded.ContentHash()
	require.NoError(t, err)
	require.Equal(t, contentAfterHistory, decodedContent)
	decodedState, err := decoded.StateHash()
	require.NoError(t, err)
	require.Equal(t, stateAfterHistory, decodedState)
}

func TestItemHashesIgnoreCreationAtOnlyInContentHash(t *testing.T) {
	item, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	contentBefore, err := item.ContentHash()
	require.NoError(t, err)
	stateBefore, err := item.StateHash()
	require.NoError(t, err)

	creationAt := *item.CreationAt + 10
	item.CreationAt = &creationAt
	contentAfter, err := item.ContentHash()
	require.NoError(t, err)
	require.Equal(t, contentBefore, contentAfter)
	stateAfter, err := item.StateHash()
	require.NoError(t, err)
	require.NotEqual(t, stateBefore, stateAfter)
}

func TestItemContentHashMatchesEquivalentHistorySnapshot(t *testing.T) {
	item, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 42))
	require.NoError(t, item.SetHistory(history))

	currentHash, err := item.ContentHash()
	require.NoError(t, err)
	snapshotData, err := json.Marshal(history.Entries[0].Item, json.Deterministic(true))
	require.NoError(t, err)
	var snapshot Item
	require.NoError(t, json.Unmarshal(snapshotData, &snapshot))
	snapshotHash, err := snapshot.ContentHash()
	require.NoError(t, err)
	require.Equal(t, currentHash, snapshotHash)
}

func TestPrepareItemEditDoesNotArchiveCreationTimestampChanges(t *testing.T) {
	previous, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	proposedSnapshot, err := previous.Snapshot()
	require.NoError(t, err)
	proposedData, err := json.Marshal(proposedSnapshot, json.Deterministic(true))
	require.NoError(t, err)
	var proposed Item
	require.NoError(t, json.Unmarshal(proposedData, &proposed))
	creationAt := *proposed.CreationAt + 10
	proposed.CreationAt = &creationAt

	updated, changed, err := PrepareItemEdit(previous, &proposed, 100, 101)
	require.NoError(t, err)
	require.False(t, changed)
	history, err := updated.History()
	require.NoError(t, err)
	require.Empty(t, history.Entries)
	require.Equal(t, *previous.ModifiedAt, *updated.ModifiedAt)
}

func TestItemHashesExcludeSummaryHashExtension(t *testing.T) {
	item, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	stateHash, err := item.StateHash()
	require.NoError(t, err)
	contentHash, err := item.ContentHash()
	require.NoError(t, err)
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"paw.pm/item-hash","algorithm":"sha256","hash":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`))
	got, err := item.StateHash()
	require.NoError(t, err)
	require.Equal(t, stateHash, got)
	got, err = item.ContentHash()
	require.NoError(t, err)
	require.Equal(t, contentHash, got)

	snapshot, err := item.Snapshot()
	require.NoError(t, err)
	data, err := json.Marshal(snapshot, json.Deterministic(true))
	require.NoError(t, err)
	require.NotContains(t, string(data), "paw.pm/item-hash")
	require.Contains(t, string(data), "paw.pm/item")
	itemData, err := json.Marshal(item, json.Deterministic(true))
	require.NoError(t, err)
	require.NotContains(t, string(itemData), "paw.pm/item-hash")
	require.Contains(t, string(item.Extensions[len(item.Extensions)-1]), "paw.pm/item-hash", "hashing and snapshotting must not mutate the source item")
}

func TestItemHashesIncludeCredentialAndUnknownExtensionChanges(t *testing.T) {
	item, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	contentBefore, err := item.ContentHash()
	require.NoError(t, err)
	stateBefore, err := item.StateHash()
	require.NoError(t, err)
	item.Credentials[0] = jsontext.Value(`{"type":"note","content":{"fieldType":"string","value":"changed synthetic secret"}}`)
	item.Extensions = append(item.Extensions, jsontext.Value(`{"name":"example.com/future","value":"opaque"}`))
	contentAfter, err := item.ContentHash()
	require.NoError(t, err)
	stateAfter, err := item.StateHash()
	require.NoError(t, err)
	require.NotEqual(t, contentBefore, contentAfter)
	require.NotEqual(t, stateBefore, stateAfter)
}

func TestItemHashUsesDeterministicMapEncoding(t *testing.T) {
	first, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	second := *first
	firstExtension, err := json.Marshal(map[string]any{"name": "example.com/future", "alpha": 1, "beta": 2}, json.Deterministic(true))
	require.NoError(t, err)
	secondExtension, err := json.Marshal(map[string]any{"beta": 2, "name": "example.com/future", "alpha": 1}, json.Deterministic(true))
	require.NoError(t, err)
	first.Extensions = append(first.Extensions, jsontext.Value(firstExtension))
	second.Extensions = append([]jsontext.Value(nil), second.Extensions...)
	second.Extensions = append(second.Extensions, jsontext.Value(secondExtension))
	firstHash, err := first.StateHash()
	require.NoError(t, err)
	secondHash, err := second.StateHash()
	require.NoError(t, err)
	require.Equal(t, firstHash, secondHash)
}

func TestItemHashReordersRawCredentialAndExtensionObjects(t *testing.T) {
	first, err := NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	second := *first
	first.Credentials = []jsontext.Value{jsontext.Value(`{"type":"note","content":{"fieldType":"string","value":"synthetic secret"},"opaque":{"large":9007199254740993}}`)}
	second.Credentials = []jsontext.Value{jsontext.Value(`{"opaque":{"large":9007199254740993},"content":{"value":"synthetic secret","fieldType":"string"},"type":"note"}`)}
	first.Extensions = append(first.Extensions, jsontext.Value(`{"name":"example.com/future","opaque":{"large":9007199254740993,"label":"kept"}}`))
	second.Extensions = append([]jsontext.Value(nil), second.Extensions...)
	second.Extensions = append(second.Extensions, jsontext.Value(`{"opaque":{"label":"kept","large":9007199254740993},"name":"example.com/future"}`))

	firstContent, err := first.ContentHash()
	require.NoError(t, err)
	secondContent, err := second.ContentHash()
	require.NoError(t, err)
	require.Equal(t, firstContent, secondContent)
	firstState, err := first.StateHash()
	require.NoError(t, err)
	secondState, err := second.StateHash()
	require.NoError(t, err)
	require.Equal(t, firstState, secondState)
}

func TestHashJSONPreservesLargeRawIntegerPrecision(t *testing.T) {
	type rawValue struct {
		Value jsontext.Value `json:"value"`
	}
	first, err := hashJSON(rawValue{Value: jsontext.Value(`9007199254740992`)})
	require.NoError(t, err)
	second, err := hashJSON(rawValue{Value: jsontext.Value(`9007199254740993`)})
	require.NoError(t, err)
	require.NotEqual(t, first, second)
}
