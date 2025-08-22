// SPDX-FileCopyrightText: 2021-2025 Luca Corbo, Paw contributors
// SPDX-License-Identifier: AGPL-3.0-or-later

package messaging

import (
	"encoding/json"
	"fmt"
	"time"

	"lucor.dev/paw/internal/paw"
)

// Declare conformity to Handler interface
var _ Handler = (*GetAppStateHandler)(nil)

type GetAppStateHandler struct {
	Storage paw.Storage
}

// Action implements browser.Handler.
func (h *GetAppStateHandler) Action() uint32 {
	return GetAppStateAction
}

type GetAppStateHandlerRequestPayload struct {
	Modified time.Time `json:"modified"`
}

type GetAppStateHandlerResponsePayload struct {
	AppStatate *paw.AppState `json:"app_state"`
}

// Serve implements browser.Handler.
func (h *GetAppStateHandler) Serve(res *Response, req *Request) {
	if req.Action != h.Action() {
		res.Error = &ActionHandlerMismatchError{ReqAction: req.Action, HandlerAction: h.Action()}
		return
	}
	res.Action = h.Action()

	v := &GetAppStateHandlerRequestPayload{}
	err := json.Unmarshal(req.Payload, v)
	if err != nil {
		res.Error = &InvalidRequestPayloadError{Got: req.Payload, Expected: v}
		return
	}

	appState, err := h.Storage.LoadAppState()
	if err != nil {
		res.Error = fmt.Errorf("unable to load application state: %w", err)
		return
	}

	if appState.Modified.After(v.Modified) {
		res.Payload = &GetAppStateHandlerResponsePayload{AppStatate: appState}
		return
	}
	res.Payload = &GetAppStateHandlerResponsePayload{AppStatate: nil}
}
