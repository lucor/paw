package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.lucor.dev/cxf"

	"lucor.dev/paw/internal/paw"
)

// TestItemJSONPreservesOpaqueData checks edits retain unknown CXF and Paw data.
func TestItemJSONPreservesOpaqueData(t *testing.T) {
	source := cxf.Item{
		ID:          "AAAAAAAAAAAAAAAAAAAAAA",
		Title:       "Example",
		Credentials: []json.RawMessage{json.RawMessage(`{"type":"future-credential","secret":"opaque"}`)},
		Extensions: []json.RawMessage{
			json.RawMessage(`{"name":"paw.pm/item","type":2,"favicon":{"host":"old.example"},"future":{"counter":9007199254740993}}`),
			json.RawMessage(`{"name":"example.com/future","secret":"opaque"}`),
		},
	}
	encoded, err := json.Marshal(source)
	require.NoError(t, err)

	var item Item
	require.NoError(t, json.Unmarshal(encoded, &item))
	require.Equal(t, paw.NoteItemType, item.Metadata.Type)
	require.Equal(t, &paw.Favicon{Host: "old.example"}, item.Metadata.Favicon)

	item.Metadata.Type = paw.LoginItemType
	item.Metadata.Favicon = &paw.Favicon{Host: "new.example"}
	item.Metadata.AddToAgent = true
	encoded, err = json.Marshal(&item)
	require.NoError(t, err)

	var decoded cxf.Item
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, source.Credentials, decoded.Credentials)
	require.Len(t, decoded.Extensions, 2)
	require.JSONEq(t, `{"name":"paw.pm/item","type":8,"favicon":{"host":"new.example"},"addToAgent":true,"future":{"counter":9007199254740993}}`, string(decoded.Extensions[0]))
	require.JSONEq(t, string(source.Extensions[1]), string(decoded.Extensions[1]))
}

// TestItemAddToAgentRoundTrip checks the SSH agent flag follows item metadata.
func TestItemAddToAgentRoundTrip(t *testing.T) {
	var item Item
	require.NoError(t, json.Unmarshal([]byte(`{"id":"AQ","title":"Key","credentials":[],"extensions":[{"name":"paw.pm/item","type":16,"future":"keep"}]}`), &item))
	require.False(t, item.Metadata.AddToAgent)
	item.Metadata.AddToAgent = true
	encoded, err := json.Marshal(&item)
	require.NoError(t, err)
	var decoded Item
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.True(t, decoded.Metadata.AddToAgent)
	decoded.Metadata.AddToAgent = false
	encoded, err = json.Marshal(&decoded)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), `"addToAgent"`)
	require.Contains(t, string(encoded), `"future":"keep"`)
}

// TestItemUnmarshalRejectsInvalidMetadata checks Paw extension requirements.
func TestItemUnmarshalRejectsInvalidMetadata(t *testing.T) {
	for _, test := range []struct {
		name       string
		extensions string
		want       string
	}{
		{name: "missing", want: "missing Paw item metadata"},
		{name: "duplicate", extensions: `[{"name":"paw.pm/item","type":2},{"name":"paw.pm/item","type":2}]`, want: "duplicate Paw item metadata"},
		{name: "invalid type", extensions: `[{"name":"paw.pm/item","type":99}]`, want: "invalid Paw item metadata"},
		{name: "missing type", extensions: `[{"name":"paw.pm/item"}]`, want: "invalid Paw item metadata"},
	} {
		t.Run(test.name, func(t *testing.T) {
			data := `{"id":"AAAAAAAAAAAAAAAAAAAAAA","title":"Example","credentials":[]`
			if test.extensions != "" {
				data += `,"extensions":` + test.extensions
			}
			data += `}`
			var item Item
			err := json.Unmarshal([]byte(data), &item)
			require.ErrorContains(t, err, test.want)
		})
	}
}

// TestItemUnmarshalIsAtomic checks failed decoding leaves the receiver unchanged.
func TestItemUnmarshalIsAtomic(t *testing.T) {
	item := Item{
		Item:     cxf.Item{ID: "AQEBAQEBAQEBAQEBAQEBAQ", Title: "Existing", Credentials: []json.RawMessage{}},
		Metadata: ItemMetadata{Type: paw.NoteItemType},
	}

	err := json.Unmarshal([]byte(`{"id":"AAAAAAAAAAAAAAAAAAAAAA","title":"Changed","credentials":[],"extensions":[{"name":"paw.pm/item","type":99}]}`), &item)

	require.Error(t, err)
	require.Equal(t, "Existing", item.Title)
	require.Equal(t, paw.NoteItemType, item.Metadata.Type)
}

// TestItemMarshalForms checks metadata edits for pointers and maps of pointers.
func TestItemMarshalForms(t *testing.T) {
	for _, existingExtension := range []bool{false, true} {
		item := Item{
			Item:     cxf.Item{ID: "AQ", Title: "Example", Credentials: []json.RawMessage{}},
			Metadata: ItemMetadata{Type: paw.LoginItemType},
		}
		if existingExtension {
			item.Extensions = []json.RawMessage{json.RawMessage(`{"name":"paw.pm/item","type":2}`)}
		}
		for _, test := range []struct {
			name   string
			value  any
			nested bool
		}{
			{name: "pointer", value: &item},
			{name: "map", value: map[string]*Item{"item": &item}, nested: true},
		} {
			t.Run(test.name, func(t *testing.T) {
				encoded, err := json.Marshal(test.value)
				require.NoError(t, err)
				if test.nested {
					var entries map[string]json.RawMessage
					require.NoError(t, json.Unmarshal(encoded, &entries))
					encoded = entries["item"]
				}
				var decoded Item
				require.NoError(t, json.Unmarshal(encoded, &decoded))
				require.Equal(t, paw.LoginItemType, decoded.Metadata.Type)
			})
		}
		// Encoding must not replace the original raw extension or add one in place.
		if existingExtension {
			require.JSONEq(t, `{"name":"paw.pm/item","type":2}`, string(item.Extensions[0]))
		} else {
			require.Nil(t, item.Extensions)
		}
	}
}

// TestItemFaviconPreservesUnknownFields checks icon edits keep future fields.
func TestItemFaviconPreservesUnknownFields(t *testing.T) {
	for _, test := range []struct {
		name    string
		favicon *paw.Favicon
	}{
		{name: "unchanged", favicon: &paw.Favicon{Host: "example.com", Data: []byte("icon"), Format: "png"}},
		{name: "edited", favicon: &paw.Favicon{Host: "new.example"}},
		{name: "cleared fields", favicon: &paw.Favicon{}},
		{name: "removed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var item Item
			require.NoError(t, json.Unmarshal([]byte(`{"id":"AQ","title":"Example","credentials":[],"extensions":[{"name":"paw.pm/item","type":2,"favicon":{"host":"example.com","data":"aWNvbg==","format":"png","future":9007199254740993}}]}`), &item))
			item.Metadata.Favicon = test.favicon
			encoded, err := json.Marshal(&item)
			require.NoError(t, err)

			var decoded Item
			require.NoError(t, json.Unmarshal(encoded, &decoded))
			require.Equal(t, test.favicon, decoded.Metadata.Favicon)
			var extension map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(decoded.Extensions[0], &extension))
			if test.favicon == nil {
				require.NotContains(t, extension, "favicon")
				return
			}
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(extension["favicon"], &fields))
			require.Equal(t, "9007199254740993", string(fields["future"]))
			require.Equal(t, test.favicon.Host != "", fields["host"] != nil)
			require.Equal(t, len(test.favicon.Data) != 0, fields["data"] != nil)
			require.Equal(t, test.favicon.Format != "", fields["format"] != nil)
		})
	}
}

// TestItemSummaryProjectsPublicFields checks vault summaries omit credentials and unrelated extensions.
func TestItemSummaryProjectsPublicFields(t *testing.T) {
	creationAt := uint64(10)
	modifiedAt := uint64(20)
	subtitle := "Work account"
	favorite := true
	item := Item{
		Item: cxf.Item{
			ID:         "AAAAAAAAAAAAAAAAAAAAAA",
			CreationAt: &creationAt,
			ModifiedAt: &modifiedAt,
			Title:      "GitHub",
			Subtitle:   &subtitle,
			Favorite:   &favorite,
			Scope: &cxf.CredentialScope{
				URLs:        []string{"https://github.com"},
				AndroidApps: []cxf.AndroidAppID{},
			},
			Credentials: []json.RawMessage{json.RawMessage(`{"type":"note","content":{"fieldType":"string","value":"private content"}}`)},
			Tags:        []string{"work"},
			Extensions: []json.RawMessage{
				json.RawMessage(`{"name":"paw.pm/item","type":4,"private":"extension secret"}`),
				json.RawMessage(`{"name":"example.com/future","secret":"unknown secret"}`),
			},
		},
		Metadata: ItemMetadata{
			Type:    paw.NoteItemType,
			Favicon: &paw.Favicon{Host: "github.com", Data: []byte("icon"), Format: "png"},
		},
	}

	summary, err := item.Summary()
	require.NoError(t, err)
	require.Equal(t, item.ID, summary.ID)
	require.Equal(t, item.CreationAt, summary.CreationAt)
	require.Equal(t, item.ModifiedAt, summary.ModifiedAt)
	require.Equal(t, item.Title, summary.Title)
	require.Equal(t, item.Subtitle, summary.Subtitle)
	require.Equal(t, item.Scope, summary.Scope)
	require.NotNil(t, summary.Credentials)
	require.Empty(t, summary.Credentials)
	require.Nil(t, summary.Favorite)
	require.Nil(t, summary.Tags)
	require.Len(t, summary.Extensions, 1)
	require.JSONEq(t, `{"name":"paw.pm/item","type":2,"favicon":{"host":"github.com","data":"aWNvbg==","format":"png"}}`, string(summary.Extensions[0]))

	encoded, err := json.Marshal(summary)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private content")
	require.NotContains(t, string(encoded), "extension secret")
	require.NotContains(t, string(encoded), "unknown secret")
	require.NotContains(t, string(encoded), `"favorite"`)
	require.NotContains(t, string(encoded), `"tags"`)

	item.Title = "GitHub work"
	renamed, err := item.Summary()
	require.NoError(t, err)
	require.Equal(t, summary.ID, renamed.ID)
	require.Equal(t, "GitHub work", renamed.Title)

	summary.Scope.URLs[0] = "https://example.com"
	require.Equal(t, "https://github.com", item.Scope.URLs[0])
}

// TestItemSummaryRejectsInvalidItem checks invalid items cannot produce summaries.
func TestItemSummaryRejectsInvalidItem(t *testing.T) {
	var item *Item
	summary, err := item.Summary()
	require.ErrorContains(t, err, "item is nil")
	require.Nil(t, summary)
}
