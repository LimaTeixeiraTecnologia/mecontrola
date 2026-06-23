package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/suite"
)

type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string   { return "i/o timeout" }
func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

type RetrySuite struct {
	suite.Suite
	ctx context.Context
}

func TestRetrySuite(t *testing.T) {
	suite.Run(t, new(RetrySuite))
}

func (s *RetrySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RetrySuite) TestIsTransientReadError() {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "deadline", err: context.DeadlineExceeded, want: true},
		{name: "wrapped_deadline", err: errors.Join(errors.New("consulta"), context.DeadlineExceeded), want: true},
		{name: "canceled", err: context.Canceled, want: false},
		{name: "net_timeout", err: fakeTimeoutError{}, want: true},
		{name: "domain", err: errors.New("amount_cents invalido"), want: false},
	}
	for _, tc := range cases {
		s.Run(tc.name, func() {
			s.Equal(tc.want, isTransientReadError(tc.err), "isTransientReadError(%v)", tc.err)
		})
	}
}

func (s *RetrySuite) TestWithReadRetry_RetriesTransientThenSucceeds() {
	calls := 0
	out, err := WithReadRetry(s.ctx, func(context.Context) (int, error) {
		calls++
		if calls < 2 {
			return 0, context.DeadlineExceeded
		}
		return 42, nil
	})
	s.Require().NoError(err, "esperava sucesso")
	s.Equal(42, out)
	s.Equal(2, calls, "esperava 2 chamadas")
}

func (s *RetrySuite) TestWithReadRetry_DoesNotRetryDomainError() {
	calls := 0
	domainErr := errors.New("categoria invalida")
	_, err := WithReadRetry(s.ctx, func(context.Context) (int, error) {
		calls++
		return 0, domainErr
	})
	s.ErrorIs(err, domainErr)
	s.Equal(1, calls, "esperava 1 chamada para erro de dominio")
}

func (s *RetrySuite) TestWithReadRetry_StopsOnCanceledContext() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()
	calls := 0
	_, err := WithReadRetry(ctx, func(context.Context) (int, error) {
		calls++
		return 0, context.DeadlineExceeded
	})
	s.Error(err, "esperava erro apos cancelamento")
	s.Equal(1, calls, "esperava 1 chamada quando ctx cancelado durante backoff")
}

func (s *RetrySuite) TestWithReadRetry_ExhaustsAttempts() {
	calls := 0
	_, err := WithReadRetry(s.ctx, func(context.Context) (int, error) {
		calls++
		return 0, context.DeadlineExceeded
	})
	s.Error(err, "esperava erro apos esgotar tentativas")
	s.Equal(maxReadRetryAttempts, calls, "esperava %d tentativas", maxReadRetryAttempts)
}
