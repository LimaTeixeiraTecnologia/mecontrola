package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/agents/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type mockWelcomeStarter struct {
	result  usecases.OnboardingResult
	err     error
	calls   int
	gotUser string
	gotPeer string
}

func (m *mockWelcomeStarter) StartOnboarding(ctx context.Context, userID, peer string) (usecases.OnboardingResult, error) {
	m.calls++
	m.gotUser = userID
	m.gotPeer = peer
	return m.result, m.err
}

type mockWelcomeDedup struct {
	inserted    bool
	insertErr   error
	insertCalls int
	deleteCalls int
	deletedIDs  []string
	insertedIDs []string
}

func (m *mockWelcomeDedup) InsertIfAbsent(ctx context.Context, eventID string) (bool, error) {
	m.insertCalls++
	m.insertedIDs = append(m.insertedIDs, eventID)
	return m.inserted, m.insertErr
}

func (m *mockWelcomeDedup) Delete(ctx context.Context, eventID string) error {
	m.deleteCalls++
	m.deletedIDs = append(m.deletedIDs, eventID)
	return nil
}

type recordingSender struct {
	err   error
	calls int
	toE1  string
	text  string
}

func (s *recordingSender) SendTextMessage(ctx context.Context, toE164, text string) error {
	s.calls++
	s.toE1 = toE164
	s.text = text
	return s.err
}

type SubscriptionBoundWelcomeConsumerSuite struct {
	suite.Suite
	ctx context.Context
	obs observability.Observability
}

func TestSubscriptionBoundWelcomeConsumerSuite(t *testing.T) {
	suite.Run(t, new(SubscriptionBoundWelcomeConsumerSuite))
}

func (s *SubscriptionBoundWelcomeConsumerSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
}

func (s *SubscriptionBoundWelcomeConsumerSuite) buildEvent(p subscriptionBoundWelcomePayload) *mockEvent {
	raw, _ := json.Marshal(p)
	return &mockEvent{
		eventType: "onboarding.subscription_bound",
		payload: outbox.Envelope{
			ID:      uuid.NewString(),
			Payload: raw,
		},
	}
}

func (s *SubscriptionBoundWelcomeConsumerSuite) validPayload() subscriptionBoundWelcomePayload {
	return subscriptionBoundWelcomePayload{
		EventID:  "evt-1",
		UserID:   "user-1",
		PeerE164: "+5511986896322",
	}
}

func (s *SubscriptionBoundWelcomeConsumerSuite) TestHandle() {
	type dependencies struct {
		starter *mockWelcomeStarter
		dedup   *mockWelcomeDedup
		sender  *recordingSender
	}

	scenarios := []struct {
		name    string
		payload subscriptionBoundWelcomePayload
		deps    dependencies
		expect  func(d dependencies, err error)
	}{
		{
			name:    "envia welcome quando dedup insere e onboarding inicia",
			payload: s.validPayload(),
			deps: dependencies{
				starter: &mockWelcomeStarter{result: usecases.OnboardingResult{Handled: true, Message: "🎉 Bem-vindo"}},
				dedup:   &mockWelcomeDedup{inserted: true},
				sender:  &recordingSender{},
			},
			expect: func(d dependencies, err error) {
				s.NoError(err)
				s.Equal(1, d.sender.calls)
				s.Equal("+5511986896322", d.sender.toE1)
				s.Contains(d.sender.text, "Bem-vindo")
				s.Equal("user-1", d.starter.gotUser)
				s.Equal(0, d.dedup.deleteCalls)
			},
		},
		{
			name:    "nao reenvia quando dedup ja processou o event_id",
			payload: s.validPayload(),
			deps: dependencies{
				starter: &mockWelcomeStarter{result: usecases.OnboardingResult{Handled: true, Message: "🎉 Bem-vindo"}},
				dedup:   &mockWelcomeDedup{inserted: false},
				sender:  &recordingSender{},
			},
			expect: func(d dependencies, err error) {
				s.NoError(err)
				s.Equal(0, d.starter.calls)
				s.Equal(0, d.sender.calls)
			},
		},
		{
			name:    "nao envia quando run ja existe (StartOnboarding Handled=false)",
			payload: s.validPayload(),
			deps: dependencies{
				starter: &mockWelcomeStarter{result: usecases.OnboardingResult{Handled: false}},
				dedup:   &mockWelcomeDedup{inserted: true},
				sender:  &recordingSender{},
			},
			expect: func(d dependencies, err error) {
				s.NoError(err)
				s.Equal(1, d.starter.calls)
				s.Equal(0, d.sender.calls)
				s.Equal(0, d.dedup.deleteCalls)
			},
		},
		{
			name:    "compensa dedup quando envio falha",
			payload: s.validPayload(),
			deps: dependencies{
				starter: &mockWelcomeStarter{result: usecases.OnboardingResult{Handled: true, Message: "🎉 Bem-vindo"}},
				dedup:   &mockWelcomeDedup{inserted: true},
				sender:  &recordingSender{err: errors.New("meta 500")},
			},
			expect: func(d dependencies, err error) {
				s.Error(err)
				s.Equal(1, d.sender.calls)
				s.Equal(1, d.dedup.deleteCalls)
				s.Equal([]string{"evt-1"}, d.dedup.deletedIDs)
			},
		},
		{
			name:    "compensa dedup quando StartOnboarding falha",
			payload: s.validPayload(),
			deps: dependencies{
				starter: &mockWelcomeStarter{err: errors.New("db down")},
				dedup:   &mockWelcomeDedup{inserted: true},
				sender:  &recordingSender{},
			},
			expect: func(d dependencies, err error) {
				s.Error(err)
				s.Equal(0, d.sender.calls)
				s.Equal(1, d.dedup.deleteCalls)
			},
		},
		{
			name:    "erro de payload incompleto sem tocar dedup",
			payload: subscriptionBoundWelcomePayload{EventID: "evt-1", UserID: "", PeerE164: "+5511986896322"},
			deps: dependencies{
				starter: &mockWelcomeStarter{},
				dedup:   &mockWelcomeDedup{inserted: true},
				sender:  &recordingSender{},
			},
			expect: func(d dependencies, err error) {
				s.Error(err)
				s.Equal(0, d.dedup.insertCalls)
				s.Equal(0, d.starter.calls)
				s.Equal(0, d.sender.calls)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			consumer := NewSubscriptionBoundWelcomeConsumer(
				scenario.deps.starter,
				scenario.deps.dedup,
				scenario.deps.sender,
				s.obs,
			)
			err := consumer.Handle(s.ctx, s.buildEvent(scenario.payload))
			scenario.expect(scenario.deps, err)
		})
	}
}
