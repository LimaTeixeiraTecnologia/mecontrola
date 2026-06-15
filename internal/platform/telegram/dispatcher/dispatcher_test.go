package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/telegram/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

type stubDedup struct {
	inserts atomic.Int32
	insert  func(botID, updateID int64) (bool, error)
}

func (s *stubDedup) InsertIfAbsent(_ context.Context, botID, updateID int64) (bool, error) {
	s.inserts.Add(1)
	if s.insert != nil {
		return s.insert(botID, updateID)
	}
	return true, nil
}

type stubResolve struct {
	resp auth.Principal
	err  error
}

func (s *stubResolve) Execute(_ context.Context, _ input.ResolvePrincipalByIdentity) (auth.Principal, error) {
	return s.resp, s.err
}

func buildPayload(t *testing.T, text string, dateOffset time.Duration) json.RawMessage {
	t.Helper()
	ts := time.Now().UTC().Add(dateOffset).Unix()
	raw := fmt.Sprintf(`{
		"update_id": 12345,
		"message": {
			"message_id": 1,
			"from": {"id": 987654321, "is_bot": false, "language_code": "pt-BR"},
			"chat": {"id": 987654321, "type": "private"},
			"date": %d,
			"text": %q
		}
	}`, ts, text)
	return json.RawMessage(raw)
}

func buildDispatcher(t *testing.T, dedup dispatcher.DedupRepository, resolve dispatcher.ResolvePrincipalUseCase, onboarding dispatcher.OnboardingRoute, agent dispatcher.AgentRoute) *dispatcher.Dispatcher {
	t.Helper()
	limiter := ratelimit.New(noop.NewProvider())
	require.NoError(t, limiter.Start(context.Background()))
	t.Cleanup(func() { _ = limiter.Shutdown(context.Background()) })
	publisher := outboxmocks.NewPublisher(t)
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Maybe()
	return dispatcher.New(11111, dedup, resolve, limiter, publisher, onboarding, agent, noop.NewProvider())
}

func TestDispatcher_Route_InvalidJSON_ReturnsInvalid(t *testing.T) {
	onboardingCalled := false
	agentCalled := false
	disp := buildDispatcher(t,
		&stubDedup{},
		&stubResolve{},
		func(context.Context, payload.Message) dispatcher.RouteOutcome {
			onboardingCalled = true
			return dispatcher.OutcomeOnboarding
		},
		func(context.Context, payload.Message) dispatcher.RouteOutcome {
			agentCalled = true
			return dispatcher.OutcomeAgent
		},
	)

	outcome, err := disp.Route(context.Background(), json.RawMessage(`{broken`))
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeInvalid, outcome)
	assert.False(t, onboardingCalled)
	assert.False(t, agentCalled)
}

func TestDispatcher_Route_NonPrivateChat_ReturnsRejected(t *testing.T) {
	disp := buildDispatcher(t, &stubDedup{}, &stubResolve{},
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeOnboarding },
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeAgent },
	)
	raw := json.RawMessage(`{
		"update_id":1,
		"message":{"message_id":1,"from":{"id":5,"is_bot":false},"chat":{"id":-100,"type":"supergroup"},"date":` + fmt.Sprintf("%d", time.Now().Unix()) + `,"text":"oi"}
	}`)

	outcome, err := disp.Route(context.Background(), raw)
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeRejected, outcome)
}

func TestDispatcher_Route_DuplicateUpdate_ReturnsDuplicate(t *testing.T) {
	dedup := &stubDedup{insert: func(_, _ int64) (bool, error) { return false, nil }}
	disp := buildDispatcher(t, dedup, &stubResolve{},
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeOnboarding },
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeAgent },
	)

	outcome, err := disp.Route(context.Background(), buildPayload(t, "oi", 0))
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeDuplicate, outcome)
}

func TestDispatcher_Route_DedupError_PropagatesError(t *testing.T) {
	dedup := &stubDedup{insert: func(_, _ int64) (bool, error) { return false, errors.New("db down") }}
	disp := buildDispatcher(t, dedup, &stubResolve{},
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeOnboarding },
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeAgent },
	)

	outcome, err := disp.Route(context.Background(), buildPayload(t, "oi", 0))
	require.Error(t, err)
	assert.Equal(t, dispatcher.OutcomeInvalid, outcome)
}

func TestDispatcher_Route_StaleTimestamp_ReturnsStale(t *testing.T) {
	disp := buildDispatcher(t, &stubDedup{}, &stubResolve{},
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeOnboarding },
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeAgent },
	)

	outcome, err := disp.Route(context.Background(), buildPayload(t, "oi", -10*time.Minute))
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeStaleTS, outcome)
}

func TestDispatcher_Route_ATIVAR_RoutesToOnboardingWithoutResolvingPrincipal(t *testing.T) {
	onboardingCalled := false
	agentCalled := false
	resolveCalled := false

	resolve := &stubResolve{}
	resolveWrapped := resolveCallCounter{inner: resolve, called: &resolveCalled}

	disp := buildDispatcher(t, &stubDedup{}, resolveWrapped,
		func(context.Context, payload.Message) dispatcher.RouteOutcome {
			onboardingCalled = true
			return dispatcher.OutcomeOnboarding
		},
		func(context.Context, payload.Message) dispatcher.RouteOutcome {
			agentCalled = true
			return dispatcher.OutcomeAgent
		},
	)

	token := "ATIVAR " + generateActivationToken(42)
	outcome, err := disp.Route(context.Background(), buildPayload(t, token, 0))
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeOnboarding, outcome)
	assert.True(t, onboardingCalled, "ATIVAR deve ir para onboarding")
	assert.False(t, agentCalled)
	assert.False(t, resolveCalled, "ATIVAR não deve resolver Principal antes (cross-link)")
}

func TestDispatcher_Route_UnknownUser_RoutesToOnboarding(t *testing.T) {
	resolve := &stubResolve{err: application.ErrUnknownUser}
	onboardingCalled := false
	disp := buildDispatcher(t, &stubDedup{}, resolve,
		func(context.Context, payload.Message) dispatcher.RouteOutcome {
			onboardingCalled = true
			return dispatcher.OutcomeOnboarding
		},
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeAgent },
	)

	outcome, err := disp.Route(context.Background(), buildPayload(t, "oi", 0))
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeOnboarding, outcome)
	assert.True(t, onboardingCalled)
}

func TestDispatcher_Route_KnownUser_RoutesToAgent(t *testing.T) {
	userID := uuid.New()
	resolve := &stubResolve{resp: auth.Principal{UserID: userID, Source: auth.SourceTelegram}}
	agentCalled := false
	disp := buildDispatcher(t, &stubDedup{}, resolve,
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeOnboarding },
		func(ctx context.Context, _ payload.Message) dispatcher.RouteOutcome {
			agentCalled = true
			p, ok := auth.FromContext(ctx)
			assert.True(t, ok)
			assert.Equal(t, userID, p.UserID)
			assert.Equal(t, auth.SourceTelegram, p.Source)
			return dispatcher.OutcomeAgent
		},
	)

	outcome, err := disp.Route(context.Background(), buildPayload(t, "Quanto gastei?", 0))
	require.NoError(t, err)
	assert.Equal(t, dispatcher.OutcomeAgent, outcome)
	assert.True(t, agentCalled)
}

func TestDispatcher_Route_ResolveDBError_Propagates(t *testing.T) {
	resolve := &stubResolve{err: errors.New("db unavailable")}
	disp := buildDispatcher(t, &stubDedup{}, resolve,
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeOnboarding },
		func(context.Context, payload.Message) dispatcher.RouteOutcome { return dispatcher.OutcomeAgent },
	)

	outcome, err := disp.Route(context.Background(), buildPayload(t, "oi", 0))
	require.Error(t, err)
	assert.Equal(t, dispatcher.OutcomeInvalid, outcome)
}

type resolveCallCounter struct {
	inner  dispatcher.ResolvePrincipalUseCase
	called *bool
}

func (r resolveCallCounter) Execute(ctx context.Context, in input.ResolvePrincipalByIdentity) (auth.Principal, error) {
	*r.called = true
	return r.inner.Execute(ctx, in)
}

func generateActivationToken(seed int) string {
	out := make([]byte, 40)
	for i := range out {
		out[i] = byte('a' + ((seed + i) % 26))
	}
	return string(out)
}
