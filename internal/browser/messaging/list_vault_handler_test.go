package messaging

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"lucor.dev/paw/internal/paw"
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
		vaults := []string{"vault1", "vault2"}
		s := &paw.StorageMock{}
		s.OnVaults = func() ([]string, error) {
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
		assert.Equal(t, vaults, payload.Vaults)
	})
}
