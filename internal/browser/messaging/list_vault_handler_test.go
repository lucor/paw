// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"lucor.dev/paw/internal/storage"
)

func TestListVaultHandler_Serve(t *testing.T) {
	t.Run("action handler mismatch", func(t *testing.T) {
		req := &Request{}
		res := &Response{}
		h := &ListVaultHandler{}
		h.Serve(res, req)
		var e *ActionHandlerMismatchError
		assert.ErrorAs(t, res.Error, &e)
	})

	t.Run("valid request", func(t *testing.T) {
		vaults := []storage.VaultInfo{{ID: "vault1", Name: "Vault One"}, {ID: "vault2", Name: "Vault Two"}}
		s := &storage.StorageMock{}
		s.OnVaults = func() ([]storage.VaultInfo, error) {
			return vaults, nil
		}
		req := &Request{Action: ListVaultAction}
		res := &Response{}
		h := &ListVaultHandler{Storage: s}
		h.Serve(res, req)
		assert.Equal(t, res.Action, req.Action)
		assert.Nil(t, res.Error)

		payload, ok := res.Payload.(*ListVaultHandlerResponsePayload)
		assert.True(t, ok)
		assert.Equal(t, []string{"Vault One", "Vault Two"}, payload.Vaults)
	})
}
