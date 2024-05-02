package messaging

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"io"
)

// sessionLifetime defines the session life
const sessionLifetime = 0

// Request represents the native message request
type Request struct {
	Action  uint32          `json:"action"`  //the action to handled
	Payload json.RawMessage `json:"payload"` //the json raw data will be unmarshaled by the action handler
}

// IsPayloadEmpty returns true if the payload contains no data
func (req *Request) IsPayloadEmpty() bool {
	if req.Payload == nil {
		return true
	}
	return bytes.Equal(req.Payload, []byte("null"))
}

// Response represents the native message response
type Response struct {
	Action  uint32 `json:"action"`  //the handled action
	Error   error  `json:"error"`   //the error occurred handling the action, if any
	Payload any    `json:"payload"` //the paylod action response to be marshaled as json
}

// Custom marshal method for Response
func (p *Response) MarshalJSON() ([]byte, error) {
	var e interface{}
	if p.Error != nil {
		e = p.Error.Error()
	}
	// Create a map to represent the custom JSON structure
	customJSON := map[string]interface{}{
		"error":   e,
		"payload": p.Payload,
	}

	return json.Marshal(customJSON)
}

// A Handler responds to an native message request.
type Handler interface {
	Serve(res *Response, req *Request)
	Action() uint32
}

func NewPawMux(h ...Handler) *mux {
	m := map[uint32]Handler{}
	for _, v := range h {
		m[v.Action()] = v
	}
	return &mux{
		m: m,
	}
}

// mux is a native messaging multiplexer for Paw
// It looks the action of each incoming request against a list of registered
// ones and calls the handler associated to the action.
type mux struct {
	m map[uint32]Handler
}

// Handle handles the native messaging request from r writing back the response
// message into w
func (pm *mux) Handle(w io.Writer, r io.Reader) error {
	req, err := readNativeMessage(r)
	if err != nil {
		return err
	}

	h, ok := pm.m[req.Action]
	if !ok {
		return &ActionNotRegisteredError{Action: req.Action}
	}

	res := &Response{}
	h.Serve(res, req)
	writeNativeMessage(w, res)
	return res.Error
}

// readNativeMessage reads the native message from the reader w as defined in
// the native messaging protocol and returns as a request if no error occur.
func readNativeMessage(r io.Reader) (*Request, error) {
	var length uint32
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return nil, ErrInvalidMessageLenght
	}

	req := &Request{}
	reader := &io.LimitedReader{R: r, N: int64(length)}
	err := json.NewDecoder(reader).Decode(req)
	return req, err
}

// writeNativeMessage writes the v to the writer w as defined in the native messaging protocol
func writeNativeMessage(w io.Writer, v any) error {
	encodedResponse, err := json.Marshal(v)
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, uint32(len(encodedResponse)))
	if err != nil {
		return err
	}
	err = binary.Write(w, binary.LittleEndian, encodedResponse)
	if err != nil {
		return err
	}
	return nil
}
