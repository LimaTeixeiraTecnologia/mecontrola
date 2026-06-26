package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	appservices "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/tools"
	appusecases "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/usecases"
	agentworkflow "github.com/LimaTeixeiraTecnologia/mecontrola/internal/agent/application/workflow"
	platformevents "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubStateChecker struct {
	snapshot appusecases.OnboardingSnapshot
	err      error
}

func (s *stubStateChecker) Load(_ context.Context, _ uuid.UUID) (appusecases.OnboardingSnapshot, error) {
	return s.snapshot, s.err
}

type stubDecisionStore struct {
	foundByMessage bool
	findErr        error
	registerErr    error
	registerCalled bool
}

func (s *stubDecisionStore) FindByMessageID(_ context.Context, _ uuid.UUID, _, _ string) (bool, error) {
	return s.foundByMessage, s.findErr
}

func (s *stubDecisionStore) RegisterGreeting(_ context.Context, _ uuid.UUID, _, _ string, _ time.Time) error {
	s.registerCalled = true
	return s.registerErr
}

type stubWelcomeMarker struct {
	alreadySent bool
	err         error
	called      bool
}

func (m *stubWelcomeMarker) MarkWelcomeSent(_ context.Context, _ uuid.UUID) (bool, error) {
	m.called = true
	return m.alreadySent, m.err
}

type OnboardingBoundConsumerSuite struct {
	suite.Suite
	ctx    context.Context
	router *stubWhatsAppRouter
}

func TestOnboardingBoundConsumerSuite(t *testing.T) {
	suite.Run(t, new(OnboardingBoundConsumerSuite))
}

func (s *OnboardingBoundConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.router = &stubWhatsAppRouter{}
}

func (s *OnboardingBoundConsumerSuite) buildEvent(userID, peer string) platformevents.Event {
	raw, _ := json.Marshal(onboardingBoundPayload{
		UserID:   userID,
		PeerE164: peer,
	})
	return &stubEvent{payload: outbox.Envelope{ID: uuid.New().String(), Payload: raw}}
}

func (s *OnboardingBoundConsumerSuite) TestHandle_ValidPayload_NoGuards_CallsRouter() {
	userID := uuid.New()
	event := s.buildEvent(userID.String(), "+5511999990000")

	sut := NewOnboardingBoundConsumer(s.router, fake.NewProvider())
	err := sut.Handle(s.ctx, event)

	s.NoError(err)
	s.True(s.router.called)
	s.Equal(userID, s.router.principal.UserID)
	s.Equal(agentworkflow.OnboardingWelcomeSignal, s.router.msg.Text)
	s.Equal("+5511999990000", s.router.msg.WhatsAppTo)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_MissingPeer_DoesNotCallRouter() {
	userID := uuid.New()
	event := s.buildEvent(userID.String(), "")

	sut := NewOnboardingBoundConsumer(s.router, fake.NewProvider())
	err := sut.Handle(s.ctx, event)

	s.NoError(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_MalformedJSON_ReturnsError() {
	event := &stubEvent{payload: outbox.Envelope{Payload: []byte("not json")}}

	sut := NewOnboardingBoundConsumer(s.router, fake.NewProvider())
	err := sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_InvalidUserID_ReturnsError() {
	event := s.buildEvent("not-a-uuid", "+5511999990000")

	sut := NewOnboardingBoundConsumer(s.router, fake.NewProvider())
	err := sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_InvalidPayloadType_ReturnsError() {
	event := &stubEvent{payload: "not_an_envelope"}

	sut := NewOnboardingBoundConsumer(s.router, fake.NewProvider())
	err := sut.Handle(s.ctx, event)

	s.Error(err)
	s.False(s.router.called)
}

func (s *OnboardingBoundConsumerSuite) TestHandle_ReplayDetected_NoOpViaDecisionStore() {
	type dependencies struct {
		decisionStore *stubDecisionStore
		stateChecker  *stubStateChecker
		welcomeMarker *stubWelcomeMarker
	}
	type args struct {
		userID string
		peer   string
	}

	scenarios := []struct {
		name         string
		args         args
		routerResult *appservices.RouteResult
		dependencies dependencies
		expect       func(router *stubWhatsAppRouter, deps dependencies, err error)
	}{
		{
			name: "replay detectado via decisionStore retorna no-op",
			args: args{userID: uuid.New().String(), peer: "+5511999990000"},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: true},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.NoError(err)
				s.False(router.called)
				s.False(deps.welcomeMarker.called)
				s.False(deps.decisionStore.registerCalled)
			},
		},
		{
			name: "welcome ja enviado via snapshot retorna no-op sem rotear (RF-29)",
			args: args{userID: uuid.New().String(), peer: "+5511999990000"},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true, WelcomeSent: true}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.NoError(err)
				s.False(router.called)
				s.False(deps.decisionStore.registerCalled)
				s.False(deps.welcomeMarker.called)
			},
		},
		{
			name: "sessao ausente retorna erro para retry (GAP-1)",
			args: args{userID: uuid.New().String(), peer: "+5511999990000"},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: false}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.Error(err)
				s.False(router.called)
			},
		},
		{
			name: "sessao em progresso executa fluxo completo e marca welcome",
			args: args{userID: uuid.New().String(), peer: "+5511999990000"},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{alreadySent: false},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.NoError(err)
				s.True(router.called)
				s.True(deps.decisionStore.registerCalled)
				s.True(deps.welcomeMarker.called)
			},
		},
		{
			name: "erro no decisionStore.FindByMessage nao bloqueia o fluxo",
			args: args{userID: uuid.New().String(), peer: "+5511999990000"},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{findErr: errors.New("db error")},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.NoError(err)
				s.True(router.called)
			},
		},
		{
			name: "erro no stateChecker retorna erro para retry",
			args: args{userID: uuid.New().String(), peer: "+5511999990000"},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false},
					stateChecker:  &stubStateChecker{err: errors.New("db error")},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.Error(err)
				s.False(router.called)
			},
		},
		{
			name:         "saudacao nao entregue (send falhou) retorna erro e nao marca welcome (F3)",
			args:         args{userID: uuid.New().String(), peer: "+5511999990000"},
			routerResult: &appservices.RouteResult{Reply: "oi", Outcome: tools.OutcomeReplyFailed, Delivered: false},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.Error(err)
				s.True(router.called)
				s.False(deps.decisionStore.registerCalled)
				s.False(deps.welcomeMarker.called)
			},
		},
		{
			name:         "saudacao com reply vazia (nao entregue) retorna erro e nao marca welcome (F3)",
			args:         args{userID: uuid.New().String(), peer: "+5511999990000"},
			routerResult: &appservices.RouteResult{Reply: "", Outcome: tools.OutcomeRouted, Delivered: false},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.Error(err)
				s.True(router.called)
				s.False(deps.decisionStore.registerCalled)
				s.False(deps.welcomeMarker.called)
			},
		},
		{
			name:         "ambas escritas de idempotencia falham retorna erro para retry (F2)",
			args:         args{userID: uuid.New().String(), peer: "+5511999990000"},
			routerResult: &appservices.RouteResult{Reply: "oi", Outcome: tools.OutcomeRouted, Delivered: true},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false, registerErr: errors.New("db error")},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{err: errors.New("db error")},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.Error(err)
				s.True(router.called)
				s.True(deps.decisionStore.registerCalled)
				s.True(deps.welcomeMarker.called)
			},
		},
		{
			name:         "apenas uma escrita de idempotencia falha preserva idempotencia e retorna nil (F2)",
			args:         args{userID: uuid.New().String(), peer: "+5511999990000"},
			routerResult: &appservices.RouteResult{Reply: "oi", Outcome: tools.OutcomeRouted, Delivered: true},
			dependencies: func() dependencies {
				return dependencies{
					decisionStore: &stubDecisionStore{foundByMessage: false, registerErr: errors.New("db error")},
					stateChecker:  &stubStateChecker{snapshot: appusecases.OnboardingSnapshot{InProgress: true}},
					welcomeMarker: &stubWelcomeMarker{},
				}
			}(),
			expect: func(router *stubWhatsAppRouter, deps dependencies, err error) {
				s.NoError(err)
				s.True(router.called)
				s.True(deps.decisionStore.registerCalled)
				s.True(deps.welcomeMarker.called)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			router := &stubWhatsAppRouter{result: scenario.routerResult}
			sut := NewOnboardingBoundConsumer(
				router,
				fake.NewProvider(),
				WithGreetingDecisionStore(scenario.dependencies.decisionStore),
				WithOnboardingStateChecker(scenario.dependencies.stateChecker),
				WithGreetingWelcomeMarker(scenario.dependencies.welcomeMarker),
			)
			event := s.buildEvent(scenario.args.userID, scenario.args.peer)
			err := sut.Handle(s.ctx, event)
			scenario.expect(router, scenario.dependencies, err)
		})
	}
}
