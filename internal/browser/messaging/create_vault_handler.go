package messaging

import (
	"encoding/json"
	"fmt"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to Handler interface
var _ Handler = (*CreateVaultHandler)(nil)

type CreateVaultHandler struct {
	Storage paw.Storage
}

// Action implements browser.Handler.
func (h *CreateVaultHandler) Action() uint32 {
	return CreateVaultAction
}

type CreateVaultHandlerRequestPayload struct {
	Vault  string `json:"vault"`
	Secret string `json:"secret"`
}

type CreateVaultHandlerResponsePayload struct {
	SessionID string `json:"session_id"`
}

// Serve implements browser.Handler.
func (h *CreateVaultHandler) Serve(res *Response, req *Request) {
	if req.Action != h.Action() {
		res.Error = &ActionHandlerMismatchError{ReqAction: req.Action, HandlerAction: h.Action()}
		return
	}
	res.Action = h.Action()

	v := &CreateVaultHandlerRequestPayload{}
	err := json.Unmarshal(req.Payload, v)
	if err != nil {
		res.Error = &InvalidRequestPayloadError{Got: req.Payload, Expected: v}
		return
	}
	s := h.Storage
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		res.Error = fmt.Errorf("paw agent not available: %w", err)
		return
	}

	key, err := s.CreateVaultKey(v.Vault, v.Secret)
	if err != nil {
		res.Error = err
		return
	}

	_, err = s.CreateVault(v.Vault, key)
	if err != nil {
		res.Error = err
		return
	}

	sessionID, err := c.Unlock(v.Vault, key, sessionLifetime)
	if err != nil {
		res.Error = fmt.Errorf("unable to unlock session: %w", err)
		return
	}

	res.Payload = &CreateVaultHandlerResponsePayload{SessionID: sessionID}
}
