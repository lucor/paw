// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import "lucor.dev/paw/internal/storage"

// Declare conformity to Handler interface
var _ Handler = (*ListVaultHandler)(nil)

type ListVaultHandler struct {
	Storage storage.Storage
}

// Action implements browser.Handler.
func (h *ListVaultHandler) Action() uint32 {
	return ListVaultAction
}

type ListVaultHandlerResponsePayload struct {
	Vaults []string `json:"vaults"`
}

// Serve implements browser.Handler.
func (h *ListVaultHandler) Serve(res *Response, req *Request) {
	if req.Action != h.Action() {
		res.Error = &ActionHandlerMismatchError{ReqAction: req.Action, HandlerAction: h.Action()}
		return
	}
	res.Action = h.Action()

	infos, err := h.Storage.Vaults()
	if err != nil {
		res.Error = err
		return
	}
	vaults := make([]string, 0, len(infos))
	for _, info := range infos {
		vaults = append(vaults, info.Name)
	}
	res.Payload = &ListVaultHandlerResponsePayload{Vaults: vaults}
}
