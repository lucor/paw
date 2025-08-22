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
var _ Handler = (*ListItemsVaultHandler)(nil)

type ListItemsVaultHandler struct {
	Storage paw.Storage
}

// Action implements browser.Handler.
func (h *ListItemsVaultHandler) Action() uint32 {
	return ListItemsVaultAction
}

type ListItemsVaultHandlerRequestPayload struct {
	Vault      string `json:"vault"`
	SessionID  string `json:"session_id"`
	FilterName string `json:"filter_name"`
	FilterType int    `json:"filter_type"`
}

type ListItemsVaultHandlerResponsePayload struct {
	Items any `json:"items"`
}

// Serve implements browser.Handler.
func (h *ListItemsVaultHandler) Serve(res *Response, req *Request) {
	if req.Action != h.Action() {
		res.Error = &ActionHandlerMismatchError{ReqAction: req.Action, HandlerAction: h.Action()}
		return
	}
	res.Action = h.Action()

	v := &ListItemsVaultHandlerRequestPayload{}
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
	key, err := c.Key(v.Vault, v.SessionID)
	if err != nil {
		res.Error = fmt.Errorf("unable to get key from agent: %w", err)
		return
	}

	vault, err := s.LoadVault(v.Vault, key)
	if err != nil {
		res.Error = fmt.Errorf("unable to vault data: %w", err)
		return
	}

	meta := vault.FilterItemMetadata(&paw.VaultFilterOptions{
		Name:     v.FilterName,
		ItemType: paw.ItemType(v.FilterType),
	})

	res.Payload = &ListItemsVaultHandlerResponsePayload{Items: meta}
}
