package valueobjects

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

var ErrClientIPInvalid = errors.New("identity: client ip invalid")

type ClientIP struct {
	ip net.IP
}

func NewClientIP(xForwardedFor string) (ClientIP, error) {
	if strings.TrimSpace(xForwardedFor) == "" {
		return ClientIP{}, nil
	}
	parts := strings.Split(xForwardedFor, ",")
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return ClientIP{}, nil
	}
	parsed := net.ParseIP(last)
	if parsed == nil {
		return ClientIP{}, fmt.Errorf("identity: %w: %q is not a valid IP", ErrClientIPInvalid, last)
	}
	return ClientIP{ip: parsed}, nil
}

func (c ClientIP) String() string {
	if c.ip == nil {
		return ""
	}
	return c.ip.String()
}

func (c ClientIP) IsZero() bool { return c.ip == nil }
