package dispatcher_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/dispatcher"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/payload"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/whatsapp/ratelimit"
)

type mockDedup struct {
	mock.Mock
}

func (m *mockDedup) InsertIfAbsent(ctx context.Context, wamid string) (bool, error) {
	args := m.Called(ctx, wamid)
	return args.Bool(0), args.Error(1)
}

type mockEstablish struct {
	mock.Mock
}

func (m *mockEstablish) Execute(ctx context.Context, in input.EstablishPrincipalInput) (auth.Principal, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(auth.Principal), args.Error(1)
}

func payloadWithTimestamp(text, ts string) json.RawMessage {
	return json.RawMessage(`{"object":"whatsapp_business_account","entry":[{"id":"1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{"display_phone_number":"5511999999999","phone_number_id":"123"},"messages":[{"from":"5511987654321","id":"wamid.test","timestamp":"` + ts + `","type":"text","text":{"body":"` + text + `"}}]}}]}]}`)
}

func validPayload(text string) json.RawMessage {
	ts := fmt.Sprintf("%d", time.Now().UTC().Unix())
	return payloadWithTimestamp(text, ts)
}

type DispatcherSuite struct {
	suite.Suite
	ctx  context.Context
	o11y *noop.Provider
}

func TestDispatcherSuite(t *testing.T) {
	suite.Run(t, new(DispatcherSuite))
}

func (s *DispatcherSuite) SetupTest() {
	s.ctx = context.Background()
	s.o11y = noop.NewProvider()
}

func (s *DispatcherSuite) newLimiter() *ratelimit.Limiter {
	return ratelimit.New(s.o11y)
}

func (s *DispatcherSuite) TestRoute() {
	userID := uuid.MustParse("a0a0a0a0-0000-0000-0000-000000000001")
	principal := auth.Principal{UserID: userID, Source: auth.SourceWhatsApp}

	scenarios := []struct {
		name                  string
		raw                   json.RawMessage
		setupDedup            func(d *mockDedup)
		setupEstablish        func(e *mockEstablish)
		setupPublisher        func(p *outboxmocks.Publisher)
		limiter               func() *ratelimit.Limiter
		activationRouteCalled *bool
		expectedOutcome       dispatcher.RouteOutcome
	}{
		{
			name: "normal message + active principal routes to agent",
			raw:  validPayload("oi"),
			setupDedup: func(d *mockDedup) {
				d.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(true, nil)
			},
			setupEstablish: func(e *mockEstablish) {
				e.On("Execute", mock.Anything, mock.Anything).Return(principal, nil)
			},
			setupPublisher:  func(p *outboxmocks.Publisher) {},
			limiter:         s.newLimiter,
			expectedOutcome: dispatcher.OutcomeAgent,
		},
		{
			name: "ErrUnknownUser invokes activationRoute",
			raw:  validPayload("oi"),
			setupDedup: func(d *mockDedup) {
				d.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(true, nil)
			},
			setupEstablish: func(e *mockEstablish) {
				e.On("Execute", mock.Anything, mock.Anything).Return(auth.Principal{}, application.ErrUnknownUser)
			},
			setupPublisher:        func(p *outboxmocks.Publisher) {},
			limiter:               s.newLimiter,
			activationRouteCalled: func() *bool { b := false; return &b }(),
			expectedOutcome:       dispatcher.OutcomeNoRoute,
		},
		{
			name: "rate-limit exceeded routes to rate_limited and publishes auth.failed",
			raw:  validPayload("oi"),
			setupDedup: func(d *mockDedup) {
				d.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(true, nil)
			},
			setupEstablish: func(e *mockEstablish) {
				e.On("Execute", mock.Anything, mock.Anything).Return(principal, nil)
			},
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.MatchedBy(func(evt outbox.Event) bool {
					return evt.AggregateUserID == userID.String()
				})).Return(nil)
			},
			limiter: func() *ratelimit.Limiter {
				l := ratelimit.New(s.o11y)
				for range ratelimit.DefaultBucketCapacity {
					l.Allow(userID)
				}
				return l
			},
			expectedOutcome: dispatcher.OutcomeRateLimited,
		},
		{
			name: "WAMID duplicate routes to duplicate without outbox event",
			raw:  validPayload("oi"),
			setupDedup: func(d *mockDedup) {
				d.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(false, nil)
			},
			setupEstablish:  func(e *mockEstablish) {},
			setupPublisher:  func(p *outboxmocks.Publisher) {},
			limiter:         s.newLimiter,
			expectedOutcome: dispatcher.OutcomeDuplicate,
		},
		{
			name:           "invalid payload routes to invalid and publishes auth.failed",
			raw:            json.RawMessage(`not-json`),
			setupDedup:     func(d *mockDedup) {},
			setupEstablish: func(e *mockEstablish) {},
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			limiter:         s.newLimiter,
			expectedOutcome: dispatcher.OutcomeInvalid,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			dedupMock := &mockDedup{}
			establishMock := &mockEstablish{}
			publisherMock := outboxmocks.NewPublisher(s.T())

			sc.setupDedup(dedupMock)
			sc.setupEstablish(establishMock)
			sc.setupPublisher(publisherMock)
			defer dedupMock.AssertExpectations(s.T())
			defer establishMock.AssertExpectations(s.T())

			limiter := sc.limiter()
			_ = limiter.Start(s.ctx)
			s.T().Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
				defer cancel()
				_ = limiter.Shutdown(ctx)
			})

			agentRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
				return dispatcher.OutcomeAgent
			}
			calledFlag := sc.activationRouteCalled
			activationRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
				if calledFlag != nil {
					*calledFlag = true
				}
				return dispatcher.OutcomeNoRoute
			}

			sut := dispatcher.New(dedupMock, establishMock, limiter, publisherMock, agentRoute, activationRoute, s.o11y)
			outcome, err := sut.Route(s.ctx, sc.raw)

			s.NoError(err)
			s.Equal(sc.expectedOutcome, outcome)
			if calledFlag != nil {
				s.True(*calledFlag, "activationRoute deve ter sido invocada")
			}
		})
	}
}

func (s *DispatcherSuite) TestRoute_NoMessages_ReturnsInvalid() {
	raw := json.RawMessage(`{"object":"whatsapp_business_account","entry":[{"id":"1","changes":[{"field":"messages","value":{"messaging_product":"whatsapp","metadata":{},"messages":[]}}]}]}`)

	dedupMock := &mockDedup{}
	establishMock := &mockEstablish{}
	publisherMock := outboxmocks.NewPublisher(s.T())

	limiter := s.newLimiter()
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	agentRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeAgent
	}
	activationRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeNoRoute
	}

	sut := dispatcher.New(dedupMock, establishMock, limiter, publisherMock, agentRoute, activationRoute, s.o11y)
	outcome, err := sut.Route(s.ctx, raw)

	s.NoError(err)
	s.Equal(dispatcher.OutcomeInvalid, outcome)
}

func (s *DispatcherSuite) TestRoute_DedupError_PropagatesErrorFor5xx() {
	raw := validPayload("oi")

	dedupMock := &mockDedup{}
	dedupMock.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(false, errors.New("db error"))
	defer dedupMock.AssertExpectations(s.T())

	establishMock := &mockEstablish{}
	publisherMock := outboxmocks.NewPublisher(s.T())

	limiter := s.newLimiter()
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	agentRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeAgent
	}
	activationRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeNoRoute
	}

	sut := dispatcher.New(dedupMock, establishMock, limiter, publisherMock, agentRoute, activationRoute, s.o11y)
	outcome, err := sut.Route(s.ctx, raw)

	s.Error(err, "dedup DB error MUST propagate para handler retornar 5xx (Meta retry)")
	s.Equal(dispatcher.OutcomeInvalid, outcome)
}

func (s *DispatcherSuite) TestRoute_EstablishPrincipal_DBError_PropagatesErrorFor5xx() {
	raw := validPayload("oi")

	dedupMock := &mockDedup{}
	dedupMock.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(true, nil)
	defer dedupMock.AssertExpectations(s.T())

	establishMock := &mockEstablish{}
	establishMock.On("Execute", mock.Anything, mock.Anything).Return(auth.Principal{}, errors.New("db unavailable"))
	defer establishMock.AssertExpectations(s.T())

	publisherMock := outboxmocks.NewPublisher(s.T())

	limiter := s.newLimiter()
	_ = limiter.Start(s.ctx)
	s.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
		defer cancel()
		_ = limiter.Shutdown(ctx)
	})

	agentRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeAgent
	}
	activationRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
		return dispatcher.OutcomeNoRoute
	}

	sut := dispatcher.New(dedupMock, establishMock, limiter, publisherMock, agentRoute, activationRoute, s.o11y)
	outcome, err := sut.Route(s.ctx, raw)

	s.Error(err, "DB error em EstablishPrincipal MUST propagar para Meta retry")
	s.Equal(dispatcher.OutcomeInvalid, outcome)
}

func (s *DispatcherSuite) TestTimestampValidation() {
	now := time.Now().UTC()

	scenarios := []struct {
		name            string
		timestamp       string
		setupPublisher  func(p *outboxmocks.Publisher)
		expectedOutcome dispatcher.RouteOutcome
	}{
		{
			name:            "timestamp within window returns no_route for unknown user",
			timestamp:       fmt.Sprintf("%d", now.Unix()),
			setupPublisher:  func(p *outboxmocks.Publisher) {},
			expectedOutcome: dispatcher.OutcomeNoRoute,
		},
		{
			name:      "timestamp +6min in future returns stale_webhook",
			timestamp: fmt.Sprintf("%d", now.Add(6*time.Minute).Unix()),
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
		{
			name:      "timestamp -6min in past returns stale_webhook",
			timestamp: fmt.Sprintf("%d", now.Add(-6*time.Minute).Unix()),
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
		{
			name:      "absent timestamp returns stale_webhook with invalid_webhook_timestamp",
			timestamp: "",
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
		{
			name:      "invalid timestamp string returns stale_webhook with invalid_webhook_timestamp",
			timestamp: "not-a-number",
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
		{
			name:      "timestamp zero treated as invalid_webhook_timestamp",
			timestamp: "0",
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
		{
			name:      "timestamp negative treated as invalid_webhook_timestamp",
			timestamp: "-1",
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
		{
			name:            "timestamp 4min59s in past stays within window returns no_route",
			timestamp:       fmt.Sprintf("%d", now.Add(-(4*time.Minute + 59*time.Second)).Unix()),
			setupPublisher:  func(p *outboxmocks.Publisher) {},
			expectedOutcome: dispatcher.OutcomeNoRoute,
		},
		{
			name:      "timestamp 5min1s in past returns stale_webhook",
			timestamp: fmt.Sprintf("%d", now.Add(-(5*time.Minute + 1*time.Second)).Unix()),
			setupPublisher: func(p *outboxmocks.Publisher) {
				p.On("Publish", mock.Anything, mock.Anything).Return(nil)
			},
			expectedOutcome: dispatcher.OutcomeStaleTS,
		},
	}

	for _, sc := range scenarios {
		s.Run(sc.name, func() {
			raw := payloadWithTimestamp("oi", sc.timestamp)

			dedupMock := &mockDedup{}
			if sc.expectedOutcome != dispatcher.OutcomeStaleTS {
				dedupMock.On("InsertIfAbsent", mock.Anything, mock.Anything).Return(true, nil)
			}
			defer dedupMock.AssertExpectations(s.T())

			establishMock := &mockEstablish{}
			publisherMock := outboxmocks.NewPublisher(s.T())
			sc.setupPublisher(publisherMock)

			if sc.expectedOutcome != dispatcher.OutcomeStaleTS {
				establishMock.On("Execute", mock.Anything, mock.Anything).Return(auth.Principal{}, application.ErrUnknownUser)
			}
			defer establishMock.AssertExpectations(s.T())

			limiter := s.newLimiter()
			_ = limiter.Start(s.ctx)
			s.T().Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), ratelimit.DefaultShutdownTimeout)
				defer cancel()
				_ = limiter.Shutdown(ctx)
			})

			agentRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
				return dispatcher.OutcomeAgent
			}
			activationRoute := func(_ context.Context, _ payload.Message) dispatcher.RouteOutcome {
				return dispatcher.OutcomeNoRoute
			}

			sut := dispatcher.New(dedupMock, establishMock, limiter, publisherMock, agentRoute, activationRoute, s.o11y)
			outcome, err := sut.Route(s.ctx, raw)

			s.NoError(err)
			s.Equal(sc.expectedOutcome, outcome)
		})
	}
}
