// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"go.lucor.dev/paw/internal/agent"
	"go.lucor.dev/paw/internal/model"
	"go.lucor.dev/paw/internal/paw"
	"go.lucor.dev/paw/internal/storage"
	"golang.org/x/net/publicsuffix"
)

// Declare conformity to Handler interface
var _ Handler = (*ListItemsVaultHandler)(nil)

type ListItemsVaultHandler struct {
	Storage storage.Storage
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

type browserItemMetadata struct {
	Name     string        `json:"name,omitempty"`
	Subtitle string        `json:"subtitle,omitempty"`
	Type     paw.ItemType  `json:"type,omitempty"`
	Modified time.Time     `json:"modified,omitempty"`
	Created  time.Time     `json:"created,omitempty"`
	Favicon  *paw.Favicon  `json:"favicon,omitempty"`
	Autofill *paw.Autofill `json:"autofill,omitempty"`
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
		res.Error = fmt.Errorf("unable to vault data: %w", err)
		return
	}

	summaries, err := vault.FilterItems(v.FilterName, paw.ItemType(v.FilterType))
	if err != nil {
		res.Error = err
		return
	}
	items := make([]browserItemMetadata, 0, len(summaries))
	for _, summary := range summaries {
		itemType, err := model.ItemTypeOf(&summary)
		if err != nil {
			res.Error = err
			return
		}
		metadata := browserItemMetadata{Name: summary.Title, Type: itemType}
		if summary.Subtitle != nil {
			metadata.Subtitle = *summary.Subtitle
		}
		if summary.ModifiedAt != nil {
			metadata.Modified = time.Unix(int64(*summary.ModifiedAt), 0).UTC()
		}
		if summary.CreationAt != nil {
			metadata.Created = time.Unix(int64(*summary.CreationAt), 0).UTC()
		}
		var extension struct {
			Favicon *paw.Favicon `json:"favicon"`
		}
		for _, raw := range summary.Extensions {
			var header struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(raw, &header) == nil && header.Name == "paw.pm/item" {
				if err := json.Unmarshal(raw, &extension); err != nil {
					res.Error = err
					return
				}
				metadata.Favicon = extension.Favicon
			}
		}
		if itemType == paw.LoginItemType && summary.Scope != nil && len(summary.Scope.URLs) > 0 {
			website, err := url.Parse(summary.Scope.URLs[0])
			if err == nil && website.Scheme == "https" && website.Hostname() != "" {
				tldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(website.Hostname())
				if err == nil {
					metadata.Autofill = &paw.Autofill{TLDPlusOne: tldPlusOne, URL: website}
				}
			}
		}
		items = append(items, metadata)
	}
	res.Payload = &ListItemsVaultHandlerResponsePayload{Items: items}
}
