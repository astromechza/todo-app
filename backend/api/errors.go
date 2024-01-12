package api

import "fmt"

type ErrCouldNotParseRequest struct {
	reason string
	inner  error
}

func (e *ErrCouldNotParseRequest) String() string {
	return fmt.Sprintf("could not parse request: %s", e.reason)
}

func (e *ErrCouldNotParseRequest) Error() string {
	return fmt.Sprintf("could not parse request: %s: %v", e.reason, e.inner)
}

func (e *ErrCouldNotParseRequest) Unwrap() error {
	return e.inner
}

func NewErrCouldNotParseRequest(reason string, inner error) *ErrCouldNotParseRequest {
	return &ErrCouldNotParseRequest{reason: reason, inner: inner}
}
