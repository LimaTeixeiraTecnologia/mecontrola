package workflows

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeTimeoutErr struct{}

func (fakeTimeoutErr) Error() string   { return "fake timeout" }
func (fakeTimeoutErr) Timeout() bool   { return true }
func (fakeTimeoutErr) Temporary() bool { return true }

func TestIsTransient(t *testing.T) {
	scenarios := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{name: "context deadline exceeded", err: context.DeadlineExceeded, want: true},
		{name: "wrapped context deadline exceeded", err: errNoted(context.DeadlineExceeded), want: true},
		{name: "sql conn done", err: sql.ErrConnDone, want: true},
		{name: "driver bad conn", err: driver.ErrBadConn, want: true},
		{name: "net timeout error", err: fakeTimeoutErr{}, want: true},
		{name: "net op error", err: &net.OpError{Op: "read", Err: errors.New("connection reset by peer")}, want: true},
		{name: "domain validation error", err: errors.New("amount_cents must be > 0"), want: false},
		{name: "category kind mismatch", err: errors.New("kind mismatch"), want: false},
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			require.Equal(t, scenario.want, IsTransient(scenario.err))
		})
	}
}

func errNoted(err error) error {
	return &noteErr{msg: "workflows.pending_entry: write", err: err}
}

type noteErr struct {
	msg string
	err error
}

func (n *noteErr) Error() string { return n.msg + ": " + n.err.Error() }
func (n *noteErr) Unwrap() error { return n.err }

func TestBackoffWithJitter_RespeitaTeto(t *testing.T) {
	for attempt := 1; attempt <= maxWriteAttempts; attempt++ {
		d := backoffWithJitter(attempt)
		require.GreaterOrEqual(t, d, time.Duration(0))
		require.LessOrEqual(t, d, writeMaxBackoff)
	}
}
