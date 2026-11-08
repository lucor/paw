package storage

import (
	"bytes"
	jsonv1 "encoding/json"
	json "encoding/json/v2"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/model"
)

// TestJSONV2ReadsLegacyEncryptedItem checks v1 plaintext remains readable through v2.
func TestJSONV2ReadsLegacyEncryptedItem(t *testing.T) {
	store, vault := newItemStoreVault(t)
	item, err := model.NewNote("Example", "synthetic secret")
	require.NoError(t, err)
	item.Metadata.Favicon = &model.Favicon{Host: "example.com", Data: []byte("icon"), Format: "png"}
	history, err := item.History()
	require.NoError(t, err)
	require.NoError(t, history.AddSnapshot(item, 100))
	require.NoError(t, item.SetHistory(history))
	require.NoError(t, store.StoreItem(vault, item))
	var encrypted bytes.Buffer
	writer, err := vault.Key().Encrypt(&encrypted)
	require.NoError(t, err)
	// Keep the v1 producer only as a compatibility fixture.
	require.NoError(t, jsonv1.NewEncoder(writer).Encode(item))
	require.NoError(t, writer.Close())
	require.NoError(t, os.WriteFile(itemPath(store, vault.Accounts[0].ID, item.ID, testItemStateHash(t, item)), encrypted.Bytes(), 0600))
	loaded, err := store.LoadItem(vault, item.ID)
	require.NoError(t, err)
	require.Equal(t, item.Metadata, loaded.Metadata)
	got, err := loaded.History()
	require.NoError(t, err)
	require.Equal(t, history, got)
}

func TestJSONV2DeterministicExtensionEncoding(t *testing.T) {
	left := map[string]int{"z": 1, "a": 2}
	right := map[string]int{}
	right["a"], right["z"] = 2, 1
	first, err := json.Marshal(left, json.Deterministic(true))
	require.NoError(t, err)
	for range 20 {
		encoded, err := json.Marshal(right, json.Deterministic(true))
		require.NoError(t, err)
		require.Equal(t, first, encoded)
	}
}
