// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"lucor.dev/paw/internal/storage"
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
		s := &storage.StorageMock{}
		s.OnVaults = func() ([]storage.VaultInfo, error) {
			return []storage.VaultInfo{}, nil
		}
		mux := NewPawMux(&ListVaultHandler{Storage: s})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		assert.Nil(t, err)
	})

	t.Run("list vaults", func(t *testing.T) {
		vaults := []storage.VaultInfo{{ID: "vault1", Name: "Vault One"}, {ID: "vault2", Name: "Vault Two"}}
		s := &storage.StorageMock{}
		s.OnVaults = func() ([]storage.VaultInfo, error) {
			return vaults, nil
		}

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
