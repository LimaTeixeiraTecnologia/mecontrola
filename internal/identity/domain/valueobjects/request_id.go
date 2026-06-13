package valueobjects

import (
	"errors"
	"fmt"
	"strings"
)

const requestIDMaxLen = 128

var ErrRequestIDInvalid = errors.New("identity: request id invalid")

type RequestID struct {
	raw string
}

func NewRequestID(raw string) (RequestID, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return RequestID{}, fmt.Errorf("identity: %w: must be non-empty", ErrRequestIDInvalid)
	}
	if len(v) > requestIDMaxLen {
		return RequestID{}, fmt.Errorf("identity: %w: exceeds max length %d", ErrRequestIDInvalid, requestIDMaxLen)
	}
	return RequestID{raw: v}, nil
}

func (r RequestID) String() string { return r.raw }

func (r RequestID) IsZero() bool { return r.raw == "" }
