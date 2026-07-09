package workflows

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"math/rand"
	"net"
	"time"
)

const (
	maxWriteAttempts = 2
	writeBaseBackoff = 100 * time.Millisecond
	writeMaxBackoff  = 900 * time.Millisecond
)

func backoffWithJitter(attempt int) time.Duration {
	exp := writeBaseBackoff
	for i := 1; i < attempt; i++ {
		exp *= 2
		if exp > writeMaxBackoff {
			exp = writeMaxBackoff
			break
		}
	}
	jitter := time.Duration(rand.Int63n(int64(exp/2 + 1))) //nolint:gosec
	result := exp/2 + jitter
	if result > writeMaxBackoff {
		return writeMaxBackoff
	}
	return result
}

func IsTransient(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, sql.ErrConnDone) || errors.Is(err, driver.ErrBadConn) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}
