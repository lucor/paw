package messaging

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidMessageLenght = errors.New("invalid message lenght")
)

type ActionNotRegisteredError struct {
	Action uint32
}

func (e *ActionNotRegisteredError) Error() string {
	return fmt.Sprintf("action %d is not registered", e.Action)
}

type ActionHandlerMismatchError struct {
	ReqAction     uint32
	HandlerAction uint32
}

func (e *ActionHandlerMismatchError) Error() string {
	return fmt.Sprintf("action handler mismatch: received %d but expected %d", e.ReqAction, e.HandlerAction)
}

type InvalidRequestPayloadError struct {
	Got      any
	Expected any
}

func (e *InvalidRequestPayloadError) Error() string {
	return fmt.Sprintf("invalid payload structure: received %#v but expected %#v", e.Got, e.Expected)
}
