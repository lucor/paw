// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	t.Run("storage error", func(t *testing.T) {
		s := newListingStorage(t)
		require.NoError(t, s.Close())
		req := &Request{Action: ListVaultAction}
		res := &Response{}
		h := &ListVaultHandler{Storage: s}
		h.Serve(res, req)
		assert.Equal(t, res.Action, req.Action)
		assert.Error(t, res.Error)
		assert.Nil(t, res.Payload)
	})
}
