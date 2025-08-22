// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"encoding/json"
	"fmt"

	"lucor.dev/paw/internal/agent"
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to Handler interface
var _ Handler = (*GetLoginItemHandler)(nil)

type GetLoginItemHandler struct {
	Storage paw.Storage
}

// Action implements browser.Handler.
func (h *GetLoginItemHandler) Action() uint32 {
	return GetLoginItem
}

type GetLoginItemHandlerRequestPayload struct {
	Vault     string `json:"vault"`
	SessionID string `json:"session_id"`
	Name      string `json:"name"`
	Type      int    `json:"type"`
}

type GetLoginItemHandlerResponsePayload struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Serve implements browser.Handler.
func (h *GetLoginItemHandler) Serve(res *Response, req *Request) {
	if req.Action != h.Action() {
		res.Error = &ActionHandlerMismatchError{ReqAction: req.Action, HandlerAction: h.Action()}
		return
	}
	res.Action = h.Action()

	v := &GetLoginItemHandlerRequestPayload{}
	err := json.Unmarshal(req.Payload, v)
	if err != nil {
		res.Error = &InvalidRequestPayloadError{Got: req.Payload, Expected: v}
		return
	}

	if paw.ItemType(v.Type) != paw.LoginItemType {
		res.Error = fmt.Errorf("invalid item type: expected %d (%s), got %d (%s)", v.Type, paw.LoginItemType, paw.ItemType(v.Type), paw.LoginItemType)
		return
	}

	s := h.Storage
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		res.Error = fmt.Errorf("paw agent not available: %w", err)
		return
	}
	key, err := c.Key(v.Vault, v.SessionID)
	if err != nil {
		res.Error = fmt.Errorf("unable to get key from agent: %w", err)
		return
	}

	vault, err := s.LoadVault(v.Vault, key)
	if err != nil {
		res.Error = fmt.Errorf("unable to load vault data: %w", err)
		return
	}

	item, err := paw.NewItem(v.Name, paw.ItemType(v.Type))
	if err != nil {
		res.Error = fmt.Errorf("unable to make item from data: %w", err)
		return
	}

	item, err = s.LoadItem(vault, item.GetMetadata())
	if err != nil {
		res.Error = fmt.Errorf("unable to load item from vault: %w", err)
		return
	}

	login := item.(*paw.Login)

	res.Payload = &GetLoginItemHandlerResponsePayload{Username: login.Username, Password: login.Password.Value}
}
