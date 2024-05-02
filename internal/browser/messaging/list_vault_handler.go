package messaging

import (
	"lucor.dev/paw/internal/paw"
)

// Declare conformity to Handler interface
var _ Handler = (*ListVaultHandler)(nil)

type ListVaultHandler struct {
	Storage paw.Storage
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

	vaults, err := h.Storage.Vaults()
	if err != nil {
		res.Error = err
		return
	}
	res.Payload = &ListVaultHandlerResponsePayload{Vaults: vaults}
}
