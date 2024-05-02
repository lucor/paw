package messaging

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"lucor.dev/paw/internal/paw"
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
		s := &paw.StorageMock{}
		s.OnVaults = func() ([]string, error) {
			return []string{}, nil
		}
		mux := NewPawMux(&ListVaultHandler{Storage: s})
		in := &bytes.Buffer{}
		out := &bytes.Buffer{}
		writeNativeMessage(in, req)
		err := mux.Handle(out, in)
		assert.Nil(t, err)
	})

	t.Run("list vaults", func(t *testing.T) {
		vaults := []string{"vault1", "vault2"}
		s := &paw.StorageMock{}
		s.OnVaults = func() ([]string, error) {
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
		assert.Equal(t, vaults, v.Vaults)
	})
}
