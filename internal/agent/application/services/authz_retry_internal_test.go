package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/domain/intent"
)

type stubParser struct{}

func (stubParser) Parse(context.Context, uuid.UUID, string) (ParsedIntent, error) {
	return ParsedIntent{}, nil
}

type stubFallback struct{}

func (stubFallback) Reply(context.Context, uuid.UUID, string, string) (string, error) {
	return "", nil
}

type stubWhatsApp struct{}

func (stubWhatsApp) SendTextMessage(context.Context, string, string) error { return nil }

type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string   { return "i/o timeout" }
func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

type AuthzRetrySuite struct {
	suite.Suite
	ctx context.Context
}

func TestAuthzRetrySuite(t *testing.T) {
	suite.Run(t, new(AuthzRetrySuite))
}

func (s *AuthzRetrySuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthzRetrySuite) newGuardRouter() *IntentRouter {
	router, err := NewIntentRouter(noop.NewProvider(), IntentRouterDeps{
		Parser:          stubParser{},
		Fallback:        stubFallback{},
		WhatsAppGateway: stubWhatsApp{},
		Location:        time.UTC,
	})
	s.Require().NoError(err, "NewIntentRouter")
	return router
}

func (s *AuthzRetrySuite) TestAuthorizeWrite_AllowsMatchingPrincipal() {
	router := s.newGuardRouter()
	owner := uuid.New()
	s.True(router.authorizeWrite(s.ctx, Principal{UserID: owner}, owner, intent.KindRecordExpense, ChannelWhatsApp), "esperava autorizacao para userID igual ao principal")
}

func (s *AuthzRetrySuite) TestAuthorizeWrite_DeniesDivergentUserID() {
	router := s.newGuardRouter()
	principal := Principal{UserID: uuid.New()}
	attacker := uuid.New()
	s.False(router.authorizeWrite(s.ctx, principal, attacker, intent.KindRecordExpense, ChannelWhatsApp), "esperava negacao quando userID efetivo diverge do principal")
}

func (s *AuthzRetrySuite) TestAuthorizeWrite_DeniesNilUserID() {
	router := s.newGuardRouter()
	s.False(router.authorizeWrite(s.ctx, Principal{UserID: uuid.Nil}, uuid.Nil, intent.KindCreateCard, ChannelWhatsApp), "esperava negacao para userID nulo")
}

func (s *AuthzRetrySuite) TestIsTransientReadError() {
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

func (s *AuthzRetrySuite) TestWithReadRetry_RetriesTransientThenSucceeds() {
	calls := 0
	out, err := withReadRetry(s.ctx, func(context.Context) (int, error) {
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

func (s *AuthzRetrySuite) TestWithReadRetry_DoesNotRetryDomainError() {
	calls := 0
	domainErr := errors.New("categoria invalida")
	_, err := withReadRetry(s.ctx, func(context.Context) (int, error) {
		calls++
		return 0, domainErr
	})
	s.ErrorIs(err, domainErr)
	s.Equal(1, calls, "esperava 1 chamada para erro de dominio")
}

func (s *AuthzRetrySuite) TestWithReadRetry_StopsOnCanceledContext() {
	ctx, cancel := context.WithCancel(s.ctx)
	cancel()
	calls := 0
	_, err := withReadRetry(ctx, func(context.Context) (int, error) {
		calls++
		return 0, context.DeadlineExceeded
	})
	s.Error(err, "esperava erro apos cancelamento")
	s.Equal(1, calls, "esperava 1 chamada quando ctx cancelado durante backoff")
}

func (s *AuthzRetrySuite) TestWithReadRetry_ExhaustsAttempts() {
	calls := 0
	_, err := withReadRetry(s.ctx, func(context.Context) (int, error) {
		calls++
		return 0, context.DeadlineExceeded
	})
	s.Error(err, "esperava erro apos esgotar tentativas")
	s.Equal(maxReadRetryAttempts, calls, "esperava %d tentativas", maxReadRetryAttempts)
}
