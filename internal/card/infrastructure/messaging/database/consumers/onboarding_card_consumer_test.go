package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/card/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubCardCreator struct {
	capturedInput input.CreateCard
	called        bool
	err           error
}

func (f *stubCardCreator) Execute(_ context.Context, in input.CreateCard) (output.Card, error) {
	f.called = true
	f.capturedInput = in
	return output.Card{}, f.err
}

type stubEvent struct {
	eventType string
	payload   any
}

func (e stubEvent) GetEventType() string { return e.eventType }
func (e stubEvent) GetPayload() any      { return e.payload }

type onboardingCardConsumerSuite struct {
	suite.Suite
}

func TestOnboardingCardConsumer(t *testing.T) {
	suite.Run(t, new(onboardingCardConsumerSuite))
}

func (s *onboardingCardConsumerSuite) buildEnvelope(userID uuid.UUID, name string, limitCents int64, closingDay, dueDay int) outbox.Envelope {
	raw, _ := json.Marshal(map[string]any{
		"UserID":     userID.String(),
		"Name":       name,
		"LimitCents": limitCents,
		"ClosingDay": closingDay,
		"DueDay":     dueDay,
	})
	return outbox.Envelope{ID: uuid.New().String(), Payload: raw}
}

func (s *onboardingCardConsumerSuite) TestHappyPath_CallsExecuteWithCorrectFields() {
	userID := uuid.New()
	env := s.buildEnvelope(userID, "Nubank", 500000, 10, 15)

	creator := &stubCardCreator{}
	consumer := consumers.NewOnboardingCardConsumer(creator, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().NoError(err)

	s.True(creator.called)
	s.Equal(userID, creator.capturedInput.UserID)
	s.Equal("Nubank", creator.capturedInput.Name)
	s.Equal("Nubank", creator.capturedInput.Nickname)
	s.Equal(int64(500000), creator.capturedInput.LimitCents)
	s.Equal(10, creator.capturedInput.ClosingDay)
	s.Equal(15, creator.capturedInput.DueDay)
}

func (s *onboardingCardConsumerSuite) TestUsecaseError_IsPropagated() {
	userID := uuid.New()
	env := s.buildEnvelope(userID, "Itau", 100000, 5, 10)

	creator := &stubCardCreator{err: errors.New("infra error")}
	consumer := consumers.NewOnboardingCardConsumer(creator, noop.NewProvider())

	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "infra error")
}

func (s *onboardingCardConsumerSuite) TestInvalidPayloadType_ReturnsError() {
	consumer := consumers.NewOnboardingCardConsumer(&stubCardCreator{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: "not-envelope"})
	s.Require().Error(err)
	s.ErrorContains(err, "tipo de payload inesperado")
}

func (s *onboardingCardConsumerSuite) TestInvalidUserID_ReturnsError() {
	raw, _ := json.Marshal(map[string]any{
		"UserID":     "not-a-uuid",
		"Name":       "Card",
		"LimitCents": 100000,
		"ClosingDay": 5,
		"DueDay":     10,
	})
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}
	consumer := consumers.NewOnboardingCardConsumer(&stubCardCreator{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "user_id inválido")
}

func (s *onboardingCardConsumerSuite) TestMalformedJSON_ReturnsError() {
	env := outbox.Envelope{ID: uuid.New().String(), Payload: []byte("not-json")}
	consumer := consumers.NewOnboardingCardConsumer(&stubCardCreator{}, noop.NewProvider())
	err := consumer.Handle(context.Background(), stubEvent{eventType: "onboarding.card_registered", payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "deserializar payload")
}
