// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"bytes"
	"encoding/base32"
	json "encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.lucor.dev/paw/internal/storage"
)

func Test_PawMux(t *testing.T) {

	t.Run("not registered action", func(t *testing.T) {
		req := &Request{Action: 100}
		mux := NewPawMux(&ListVaultHandler{})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		var eia *ActionNotRegisteredError
		assert.ErrorAs(t, err, &eia)
	})

	t.Run("registered action", func(t *testing.T) {
		req := &Request{Action: ListVaultAction}
		s := newListingStorage(t)
		mux := NewPawMux(&ListVaultHandler{Storage: s})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		assert.Nil(t, err)
	})

	t.Run("list vaults", func(t *testing.T) {
		s := newListingStorage(t, "Vault One", "Vault Two")

		req := &Request{Action: ListVaultAction}
		mux := NewPawMux(&ListVaultHandler{Storage: s})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		assert.Nil(t, err)
		assert.NotEmpty(t, out)

		res, err := readNativeMessage(out)
		assert.Nil(t, err)
		v := &ListVaultHandlerResponsePayload{}
		err = json.Unmarshal(res.Payload, v)
		assert.Nil(t, err)
		assert.Equal(t, []string{"Vault One", "Vault Two"}, v.Vaults)
	})
}

// newListingStorage creates only the public metadata needed by vault listing.
func newListingStorage(t *testing.T, names ...string) *storage.FileStorage {
	t.Helper()
	s, err := storage.NewFileStorage(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	for i, name := range names {
		var id [16]byte
		id[15] = byte(i)
		directory := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(id[:]))
		path := filepath.Join(s.Root(), "storage", directory)
		require.NoError(t, os.Mkdir(path, 0o700))
		data, err := json.Marshal(map[string]string{"name": name}, json.Deterministic(true))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(path, "metadata.json"), data, 0o600))
	}
	return s
}
