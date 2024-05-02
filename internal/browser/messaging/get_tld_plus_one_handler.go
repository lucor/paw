package messaging

import (
	"encoding/json"
	"fmt"

	"golang.org/x/net/publicsuffix"
)

// Declare conformity to Handler interface
var _ Handler = (*GetTLDPlusOneHandler)(nil)

type GetTLDPlusOneHandler struct {
}

// Action implements browser.Handler.
func (h *GetTLDPlusOneHandler) Action() uint32 {
	return GetTLDPlusOneAction
}

type GetTLDPlusOneHandlerRequestPayload struct {
	Hostname string `json:"hostname"`
	Type     int    `json:"type"`
}

type GetTLDPlusOneHandlerResponsePayload struct {
	TldPlusOne string `json:"tld_plus_one"`
}

// Serve implements browser.Handler.
func (h *GetTLDPlusOneHandler) Serve(res *Response, req *Request) {
	if req.Action != h.Action() {
		res.Error = &ActionHandlerMismatchError{ReqAction: req.Action, HandlerAction: h.Action()}
		return
	}
	res.Action = h.Action()

	v := &GetTLDPlusOneHandlerRequestPayload{}
	err := json.Unmarshal(req.Payload, v)
	if err != nil {
		res.Error = &InvalidRequestPayloadError{Got: req.Payload, Expected: v}
		return
	}

	tldPlusOne, err := publicsuffix.EffectiveTLDPlusOne(v.Hostname)
	if err != nil {
		res.Error = fmt.Errorf("unable to get tld plus one: %w", err)
		return
	}

	res.Payload = &GetTLDPlusOneHandlerResponsePayload{TldPlusOne: tldPlusOne}
}
