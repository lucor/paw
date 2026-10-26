// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"encoding/json"
	"fmt"

	"go.lucor.dev/cxf"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
)

// Declare conformity to Handler interface
var _ Handler = (*GetLoginItemHandler)(nil)

type GetLoginItemHandler struct {
	Storage storage.Storage
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
	info, err := resolveVault(s, v.Vault)
	if err != nil {
		res.Error = err
		return
	}
	c, err := agent.NewClient(s.SocketAgentPath())
	if err != nil {
		res.Error = fmt.Errorf("paw agent not available: %w", err)
		return
	}
	key, err := c.Key(info.ID, v.SessionID)
	if err != nil {
		res.Error = fmt.Errorf("unable to get key from agent: %w", err)
		return
	}

	vault, err := s.LoadVault(info.ID, key)
	if err != nil {
		res.Error = fmt.Errorf("unable to load vault data: %w", err)
		return
	}

	var summary *cxf.Item
	for index := range vault.Accounts[0].Items {
		candidate := &vault.Accounts[0].Items[index]
		itemType, err := model.ItemTypeOf(candidate)
		if err != nil {
			res.Error = err
			return
		}
		if candidate.Title == v.Name && itemType == paw.LoginItemType {
			if summary != nil {
				res.Error = fmt.Errorf("login item %q is ambiguous", v.Name)
				return
			}
			summary = candidate
		}
	}
	if summary == nil {
		res.Error = fmt.Errorf("login item %q does not exist", v.Name)
		return
	}
	item, err := s.LoadItem(vault, summary.ID)
	if err != nil {
		res.Error = fmt.Errorf("unable to load item from vault: %w", err)
		return
	}
	for _, raw := range item.Credentials {
		typeName, err := cxf.CredentialTypeOf(raw)
		if err != nil {
			res.Error = err
			return
		}
		if typeName != cxf.CredentialTypeBasicAuth {
			continue
		}
		var login cxf.BasicAuth
		if err := json.Unmarshal(raw, &login); err != nil {
			res.Error = err
			return
		}
		response := GetLoginItemHandlerResponsePayload{}
		if login.Username != nil {
			response.Username = login.Username.Value
		}
		if login.Password != nil {
			response.Password = login.Password.Value
		}
		res.Payload = &response
		return
	}
	res.Error = fmt.Errorf("login item %q has no basic-auth credential", v.Name)
}
