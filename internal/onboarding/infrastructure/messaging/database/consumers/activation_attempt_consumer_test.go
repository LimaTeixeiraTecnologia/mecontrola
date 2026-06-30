package consumers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeActivateFromInbound struct {
	callCount int
	returnErr error
	returnOut usecases.ActivateFromInboundResult
	lastInput input.ActivateFromInboundInput
}

func (f *fakeActivateFromInbound) Execute(_ context.Context, in input.ActivateFromInboundInput) (usecases.ActivateFromInboundResult, error) {
	f.callCount++
	f.lastInput = in
	return f.returnOut, f.returnErr
}

type activationAttemptEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *activationAttemptEvent) GetEventType() string { return e.eventType }
func (e *activationAttemptEvent) GetPayload() any      { return e.envelope }

func newActivationAttemptEvent(payload any) events.Event {
	raw, _ := json.Marshal(payload)
	return &activationAttemptEvent{
		eventType: "onboarding.activation.attempted.v1",
		envelope: outbox.Envelope{
			ID:        "evt-attempt-001",
			EventType: "onboarding.activation.attempted.v1",
			Payload:   json.RawMessage(raw),
		},
	}
}

type badPayloadEvent struct{}

func (e *badPayloadEvent) GetEventType() string { return "onboarding.activation.attempted.v1" }
func (e *badPayloadEvent) GetPayload() any      { return "not-an-envelope" }

type ActivationAttemptConsumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestActivationAttemptConsumerSuite(t *testing.T) {
	suite.Run(t, new(ActivationAttemptConsumerSuite))
}

func (s *ActivationAttemptConsumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ActivationAttemptConsumerSuite) TestHandle() {
	type args struct {
		event events.Event
	}
	type dependencies struct {
		uc *fakeActivateFromInbound
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(*fakeActivateFromInbound, error)
	}{
		{
			name: "deve delegar ao usecase com payload valido",
			args: args{
				event: newActivationAttemptEvent(map[string]any{
					"peer_e164":  "+5511999999999",
					"text":       "Oi",
					"message_id": "wamid-001",
				}),
			},
			dependencies: dependencies{uc: func() *fakeActivateFromInbound {
				return &fakeActivateFromInbound{
					returnOut: usecases.ActivateFromInboundResult{Outcome: usecases.ActivateOutcomePhoneMatched},
				}
			}()},
			expect: func(uc *fakeActivateFromInbound, err error) {
				s.NoError(err)
				s.Equal(1, uc.callCount)
				s.Equal("+5511999999999", uc.lastInput.PeerE164)
				s.Equal("Oi", uc.lastInput.Text)
				s.Equal("wamid-001", uc.lastInput.MessageID)
			},
		},
		{
			name: "deve ignorar evento sem peer_e164",
			args: args{
				event: newActivationAttemptEvent(map[string]any{
					"peer_e164":  "",
					"text":       "Oi",
					"message_id": "wamid-002",
				}),
			},
			dependencies: dependencies{uc: &fakeActivateFromInbound{}},
			expect: func(uc *fakeActivateFromInbound, err error) {
				s.NoError(err)
				s.Equal(0, uc.callCount)
			},
		},
		{
			name: "deve propagar erro do usecase",
			args: args{
				event: newActivationAttemptEvent(map[string]any{
					"peer_e164":  "+5511999999999",
					"text":       "Oi",
					"message_id": "wamid-003",
				}),
			},
			dependencies: dependencies{uc: func() *fakeActivateFromInbound {
				return &fakeActivateFromInbound{returnErr: errors.New("db error")}
			}()},
			expect: func(uc *fakeActivateFromInbound, err error) {
				s.ErrorContains(err, "activation_attempt")
				s.Equal(1, uc.callCount)
			},
		},
		{
			name:         "deve retornar erro para payload inesperado",
			args:         args{event: &badPayloadEvent{}},
			dependencies: dependencies{uc: &fakeActivateFromInbound{}},
			expect: func(uc *fakeActivateFromInbound, err error) {
				s.ErrorContains(err, "unexpected payload type")
				s.Equal(0, uc.callCount)
			},
		},
		{
			name: "deve completar sem erro em already_active (idempotencia)",
			args: args{
				event: newActivationAttemptEvent(map[string]any{
					"peer_e164":  "+5511999999999",
					"text":       "Oi",
					"message_id": "wamid-004",
				}),
			},
			dependencies: dependencies{uc: func() *fakeActivateFromInbound {
				return &fakeActivateFromInbound{
					returnOut: usecases.ActivateFromInboundResult{Outcome: usecases.ActivateOutcomeAlreadyActive},
				}
			}()},
			expect: func(uc *fakeActivateFromInbound, err error) {
				s.NoError(err)
				s.Equal(1, uc.callCount)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			obs := fake.NewProvider()
			consumer := NewActivationAttemptConsumer(scenario.dependencies.uc, obs)
			scenario.expect(scenario.dependencies.uc, consumer.Handle(s.ctx, scenario.args.event))
		})
	}
}
