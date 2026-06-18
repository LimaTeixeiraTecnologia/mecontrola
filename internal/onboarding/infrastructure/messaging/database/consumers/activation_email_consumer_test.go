package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeSendActivationEmailUseCase struct {
	callCount int
	returnErr error
	lastInput usecases.SendActivationEmailInput
}

func (f *fakeSendActivationEmailUseCase) Execute(_ context.Context, in usecases.SendActivationEmailInput) error {
	f.callCount++
	f.lastInput = in
	return f.returnErr
}

type activationEmailEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *activationEmailEvent) GetEventType() string { return e.eventType }
func (e *activationEmailEvent) GetPayload() any      { return e.envelope }

func newActivationEmailEvent(payload any) events.Event {
	rawPayload, _ := json.Marshal(payload)
	return &activationEmailEvent{
		eventType: "billing.subscription.activated",
		envelope: outbox.Envelope{
			ID:        "evt-act-001",
			EventType: "billing.subscription.activated",
			Payload:   json.RawMessage(rawPayload),
		},
	}
}

type activationEmailBadPayloadEvent struct{}

func (e *activationEmailBadPayloadEvent) GetEventType() string {
	return "billing.subscription.activated"
}
func (e *activationEmailBadPayloadEvent) GetPayload() any { return "not-an-envelope" }

type ActivationEmailConsumerSuite struct {
	suite.Suite
	ctx context.Context
}

func TestActivationEmailConsumerSuite(t *testing.T) {
	suite.Run(t, new(ActivationEmailConsumerSuite))
}

func (s *ActivationEmailConsumerSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ActivationEmailConsumerSuite) TestHandle() {
	occurredAt := time.Now().UTC().Truncate(time.Second)

	type args struct {
		event events.Event
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(*fakeSendActivationEmailUseCase)
		expect func(*fakeSendActivationEmailUseCase, error)
	}{
		{
			name: "deve chamar sendActivationEmail com funnel_token e customer_email corretos",
			args: args{
				event: newActivationEmailEvent(map[string]any{
					"subscription_id":      "sub-001",
					"funnel_token":         "token-sck-abc",
					"plan_code":            "MONTHLY",
					"external_sale_id":     "sale-001",
					"customer_mobile_e164": "+5511999999999",
					"customer_email":       "user@example.com",
					"paid_at":              occurredAt,
					"occurred_at":          occurredAt,
				}),
			},
			setup: func(uc *fakeSendActivationEmailUseCase) {},
			expect: func(uc *fakeSendActivationEmailUseCase, err error) {
				s.NoError(err)
				s.Equal(1, uc.callCount)
				s.Equal("token-sck-abc", uc.lastInput.ClearToken)
				s.Equal("user@example.com", uc.lastInput.CustomerEmail)
			},
		},
		{
			name: "deve ignorar evento sem funnel_token",
			args: args{
				event: newActivationEmailEvent(map[string]any{
					"subscription_id":  "sub-002",
					"funnel_token":     "",
					"customer_email":   "user@example.com",
					"external_sale_id": "sale-002",
					"occurred_at":      occurredAt,
				}),
			},
			setup: func(uc *fakeSendActivationEmailUseCase) {},
			expect: func(uc *fakeSendActivationEmailUseCase, err error) {
				s.NoError(err)
				s.Equal(0, uc.callCount)
			},
		},
		{
			name: "deve ignorar evento sem customer_email",
			args: args{
				event: newActivationEmailEvent(map[string]any{
					"subscription_id":  "sub-003",
					"funnel_token":     "token-sck-xyz",
					"customer_email":   "",
					"external_sale_id": "sale-003",
					"occurred_at":      occurredAt,
				}),
			},
			setup: func(uc *fakeSendActivationEmailUseCase) {},
			expect: func(uc *fakeSendActivationEmailUseCase, err error) {
				s.NoError(err)
				s.Equal(0, uc.callCount)
			},
		},
		{
			name: "deve propagar erro do use case",
			args: args{
				event: newActivationEmailEvent(map[string]any{
					"funnel_token":   "token-err",
					"customer_email": "err@example.com",
					"occurred_at":    occurredAt,
				}),
			},
			setup: func(uc *fakeSendActivationEmailUseCase) {
				uc.returnErr = errors.New("smtp error")
			},
			expect: func(uc *fakeSendActivationEmailUseCase, err error) {
				s.ErrorContains(err, "activation_email")
			},
		},
		{
			name:  "deve retornar erro para payload inesperado",
			args:  args{event: &activationEmailBadPayloadEvent{}},
			setup: func(uc *fakeSendActivationEmailUseCase) {},
			expect: func(uc *fakeSendActivationEmailUseCase, err error) {
				s.ErrorContains(err, "unexpected payload type")
				s.Equal(0, uc.callCount)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			uc := &fakeSendActivationEmailUseCase{}
			scenario.setup(uc)
			consumer := consumers.NewActivationEmailConsumer(uc, noop.NewProvider())
			scenario.expect(uc, consumer.Handle(s.ctx, scenario.args.event))
		})
	}
}
