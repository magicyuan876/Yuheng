package client

import (
	"errors"
	"fmt"
)

// ErrSSEStreamTerminal marks a terminal response_type=error, done=true frame
// from an SSE stream. The stream delivered the frame to the callback before
// returning the error.
var ErrSSEStreamTerminal = errors.New("SSE stream terminal error")

// SSEStreamError is returned when the server emits a terminal error frame on
// an SSE stream.
type SSEStreamError struct {
	Content string
}

func (e *SSEStreamError) Error() string {
	return fmt.Sprintf("SSE stream error: %s", e.Content)
}

func (e *SSEStreamError) Unwrap() error {
	return ErrSSEStreamTerminal
}

// NewSSEStreamError builds the terminal SSE stream error returned by the
// streaming readers after delivering the error frame to the callback.
func NewSSEStreamError(content string) error {
	return &SSEStreamError{Content: content}
}

// IsSSEStreamError reports whether err is, or wraps, a terminal SSE stream
// error returned by the SDK's streaming readers.
func IsSSEStreamError(err error) bool {
	var sse *SSEStreamError
	return errors.As(err, &sse)
}
