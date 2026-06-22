package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"

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

func newGuardRouter(t *testing.T) *IntentRouter {
	t.Helper()
	router, err := NewIntentRouter(noop.NewProvider(), IntentRouterDeps{
		Parser:          stubParser{},
		Fallback:        stubFallback{},
		WhatsAppGateway: stubWhatsApp{},
		Location:        time.UTC,
	})
	if err != nil {
		t.Fatalf("NewIntentRouter: %v", err)
	}
	return router
}

func TestAuthorizeWrite_AllowsMatchingPrincipal(t *testing.T) {
	t.Parallel()
	router := newGuardRouter(t)
	owner := uuid.New()
	if !router.authorizeWrite(context.Background(), Principal{UserID: owner}, owner, intent.KindLogExpense, ChannelWhatsApp) {
		t.Fatal("esperava autorizacao para userID igual ao principal")
	}
}

func TestAuthorizeWrite_DeniesDivergentUserID(t *testing.T) {
	t.Parallel()
	router := newGuardRouter(t)
	principal := Principal{UserID: uuid.New()}
	attacker := uuid.New()
	if router.authorizeWrite(context.Background(), principal, attacker, intent.KindLogExpense, ChannelWhatsApp) {
		t.Fatal("esperava negacao quando userID efetivo diverge do principal")
	}
}

func TestAuthorizeWrite_DeniesNilUserID(t *testing.T) {
	t.Parallel()
	router := newGuardRouter(t)
	if router.authorizeWrite(context.Background(), Principal{UserID: uuid.Nil}, uuid.Nil, intent.KindCreateCard, ChannelWhatsApp) {
		t.Fatal("esperava negacao para userID nulo")
	}
}

type fakeTimeoutError struct{}

func (fakeTimeoutError) Error() string   { return "i/o timeout" }
func (fakeTimeoutError) Timeout() bool   { return true }
func (fakeTimeoutError) Temporary() bool { return true }

func TestIsTransientReadError(t *testing.T) {
	t.Parallel()
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
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isTransientReadError(tc.err); got != tc.want {
				t.Fatalf("isTransientReadError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestWithReadRetry_RetriesTransientThenSucceeds(t *testing.T) {
	t.Parallel()
	calls := 0
	out, err := withReadRetry(context.Background(), func(context.Context) (int, error) {
		calls++
		if calls < 2 {
			return 0, context.DeadlineExceeded
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("esperava sucesso, recebeu erro: %v", err)
	}
	if out != 42 {
		t.Fatalf("esperava 42, recebeu %d", out)
	}
	if calls != 2 {
		t.Fatalf("esperava 2 chamadas, recebeu %d", calls)
	}
}

func TestWithReadRetry_DoesNotRetryDomainError(t *testing.T) {
	t.Parallel()
	calls := 0
	domainErr := errors.New("categoria invalida")
	_, err := withReadRetry(context.Background(), func(context.Context) (int, error) {
		calls++
		return 0, domainErr
	})
	if !errors.Is(err, domainErr) {
		t.Fatalf("esperava erro de dominio, recebeu %v", err)
	}
	if calls != 1 {
		t.Fatalf("esperava 1 chamada para erro de dominio, recebeu %d", calls)
	}
}

func TestWithReadRetry_StopsOnCanceledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	_, err := withReadRetry(ctx, func(context.Context) (int, error) {
		calls++
		return 0, context.DeadlineExceeded
	})
	if err == nil {
		t.Fatal("esperava erro apos cancelamento")
	}
	if calls != 1 {
		t.Fatalf("esperava 1 chamada quando ctx cancelado durante backoff, recebeu %d", calls)
	}
}

func TestWithReadRetry_ExhaustsAttempts(t *testing.T) {
	t.Parallel()
	calls := 0
	_, err := withReadRetry(context.Background(), func(context.Context) (int, error) {
		calls++
		return 0, context.DeadlineExceeded
	})
	if err == nil {
		t.Fatal("esperava erro apos esgotar tentativas")
	}
	if calls != maxReadRetryAttempts {
		t.Fatalf("esperava %d tentativas, recebeu %d", maxReadRetryAttempts, calls)
	}
}
