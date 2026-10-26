// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"encoding/json"
	"fmt"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/storage"
)

// Declare conformity to Handler interface
var _ Handler = (*CreateVaultHandler)(nil)

type CreateVaultHandler struct {
	Storage storage.Storage
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

	vault, err := s.CreateVault(v.Vault, v.Secret)
	if err != nil {
		res.Error = err
		return
	}

	sessionID, err := c.Unlock(vault.Accounts[0].ID, vault.Key(), sessionLifetime)
	if err != nil {
		res.Error = fmt.Errorf("unable to unlock session: %w", err)
		return
	}

	res.Payload = &CreateVaultHandlerResponsePayload{SessionID: sessionID}
}
