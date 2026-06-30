package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeWelcomeGateway struct {
	sent      []string
	returnErr error
	callCount int
}

func (f *fakeWelcomeGateway) SendTextMessage(_ context.Context, _ string, text string) error {
	f.callCount++
	if f.returnErr != nil {
		return f.returnErr
	}
	f.sent = append(f.sent, text)
	return nil
}

type fakeWelcomeDedup struct {
	inserted bool
	err      error
	calls    int
}

func (f *fakeWelcomeDedup) InsertIfAbsent(_ context.Context, _ string) (bool, error) {
	f.calls++
	return f.inserted, f.err
}

type welcomeEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *welcomeEvent) GetEventType() string { return e.eventType }
func (e *welcomeEvent) GetPayload() any      { return e.envelope }

func newWelcomeEvent(payload any) events.Event {
	raw, _ := json.Marshal(payload)
	return &welcomeEvent{
		eventType: "onboarding.subscription_bound",
		envelope: outbox.Envelope{
			ID:        "evt-bound-001",
			EventType: "onboarding.subscription_bound",
			Payload:   json.RawMessage(raw),
		},
	}
}

type welcomeBadPayloadEvent struct{}

func (e *welcomeBadPayloadEvent) GetEventType() string { return "onboarding.subscription_bound" }
func (e *welcomeBadPayloadEvent) GetPayload() any      { return "not-an-envelope" }

type WelcomeConsumerSuite struct {
	suite.Suite
	ctx              context.Context
	activationWindow time.Duration
}

func TestWelcomeConsumerSuite(t *testing.T) {
	suite.Run(t, new(WelcomeConsumerSuite))
}

func (s *WelcomeConsumerSuite) SetupTest() {
	s.ctx = context.Background()
	s.activationWindow = 24 * time.Hour
}

func (s *WelcomeConsumerSuite) TestHandle() {
	recentBoundAt := time.Now().UTC().Add(-5 * time.Minute)
	oldBoundAt := time.Now().UTC().Add(-25 * time.Hour)

	type args struct {
		event events.Event
	}
	type dependencies struct {
		gw    *fakeWelcomeGateway
		dedup *fakeWelcomeDedup
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(*fakeWelcomeGateway, error)
	}{
		{
			name: "deve enviar ambas as mensagens dentro da janela",
			args: args{
				event: newWelcomeEvent(map[string]any{
					"peer_e164": "+5511999999999",
					"bound_at":  recentBoundAt,
				}),
			},
			dependencies: dependencies{gw: &fakeWelcomeGateway{}},
			expect: func(gw *fakeWelcomeGateway, err error) {
				s.NoError(err)
				s.Equal(2, gw.callCount)
				s.Equal([]string{"welcome msg", "intro msg"}, gw.sent)
			},
		},
		{
			name: "deve descartar evento fora da janela de ativacao",
			args: args{
				event: newWelcomeEvent(map[string]any{
					"peer_e164": "+5511999999999",
					"bound_at":  oldBoundAt,
				}),
			},
			dependencies: dependencies{gw: &fakeWelcomeGateway{}},
			expect: func(gw *fakeWelcomeGateway, err error) {
				s.NoError(err)
				s.Equal(0, gw.callCount)
			},
		},
		{
			name: "deve ignorar evento sem peer_e164",
			args: args{
				event: newWelcomeEvent(map[string]any{
					"peer_e164": "",
					"bound_at":  recentBoundAt,
				}),
			},
			dependencies: dependencies{gw: &fakeWelcomeGateway{}},
			expect: func(gw *fakeWelcomeGateway, err error) {
				s.NoError(err)
				s.Equal(0, gw.callCount)
			},
		},
		{
			name:         "deve retornar erro para payload inesperado",
			args:         args{event: &welcomeBadPayloadEvent{}},
			dependencies: dependencies{gw: &fakeWelcomeGateway{}},
			expect: func(gw *fakeWelcomeGateway, err error) {
				s.ErrorContains(err, "unexpected payload type")
				s.Equal(0, gw.callCount)
			},
		},
		{
			name: "deve pular envio em reentrega do mesmo evento (RF-27 idempotencia)",
			args: args{
				event: newWelcomeEvent(map[string]any{
					"peer_e164": "+5511999999999",
					"bound_at":  recentBoundAt,
				}),
			},
			dependencies: dependencies{
				gw:    &fakeWelcomeGateway{},
				dedup: &fakeWelcomeDedup{inserted: false},
			},
			expect: func(gw *fakeWelcomeGateway, err error) {
				s.NoError(err)
				s.Equal(0, gw.callCount)
			},
		},
		{
			name: "deve propagar erro no envio da mensagem de boas-vindas",
			args: args{
				event: newWelcomeEvent(map[string]any{
					"peer_e164": "+5511999999999",
					"bound_at":  recentBoundAt,
				}),
			},
			dependencies: dependencies{gw: func() *fakeWelcomeGateway {
				return &fakeWelcomeGateway{returnErr: errors.New("meta api error")}
			}()},
			expect: func(gw *fakeWelcomeGateway, err error) {
				s.ErrorContains(err, "send welcome")
				s.Equal(1, gw.callCount)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			obs := fake.NewProvider()
			dedup := scenario.dependencies.dedup
			if dedup == nil {
				dedup = &fakeWelcomeDedup{inserted: true}
			}
			consumer := NewWelcomeConsumer(
				scenario.dependencies.gw,
				dedup,
				"welcome msg",
				"intro msg",
				s.activationWindow,
				obs,
			)
			scenario.expect(scenario.dependencies.gw, consumer.Handle(s.ctx, scenario.args.event))
		})
	}
}
