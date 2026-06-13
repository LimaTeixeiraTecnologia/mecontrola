package valueobjects

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

var (
	ErrGatewayTimestampInvalid = errors.New("identity: gateway timestamp invalid format")
	ErrGatewayTimestampStale   = errors.New("identity: gateway timestamp stale")
)

type GatewayTimestamp struct {
	at  time.Time
	raw string
}

func NewGatewayTimestamp(raw string, now time.Time, window time.Duration) (GatewayTimestamp, error) {
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return GatewayTimestamp{}, fmt.Errorf("identity: %w: not a valid unix seconds integer", ErrGatewayTimestampInvalid)
	}
	parsed := time.Unix(secs, 0).UTC()
	diff := now.Sub(parsed)
	if diff < 0 {
		diff = -diff
	}
	if diff > window {
		return GatewayTimestamp{}, fmt.Errorf("identity: %w: delta %s exceeds window %s", ErrGatewayTimestampStale, diff, window)
	}
	return GatewayTimestamp{at: parsed, raw: raw}, nil
}

func (t GatewayTimestamp) Time() time.Time {
	return t.at
}

func (t GatewayTimestamp) Raw() string {
	return t.raw
}
