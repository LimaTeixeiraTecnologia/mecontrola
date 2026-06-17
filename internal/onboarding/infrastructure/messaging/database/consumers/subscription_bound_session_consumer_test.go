package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/onboarding/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type fakeStartBudgetUC struct {
	called bool
	input  usecases.StartBudgetConfigurationInput
	result usecases.StartBudgetConfigurationResult
	err    error
}

func (f *fakeStartBudgetUC) Execute(_ context.Context, in usecases.StartBudgetConfigurationInput) (usecases.StartBudgetConfigurationResult, error) {
	f.called = true
	f.input = in
	return f.result, f.err
}

type subscriptionBoundStubEvent struct {
	payload any
}

func (e subscriptionBoundStubEvent) GetEventType() string {
	return "onboarding.subscription_bound"
}
func (e subscriptionBoundStubEvent) GetPayload() any { return e.payload }

type SubscriptionBoundSessionConsumerSuite struct {
	suite.Suite
}

func TestSubscriptionBoundSessionConsumer(t *testing.T) {
	suite.Run(t, new(SubscriptionBoundSessionConsumerSuite))
}

func (s *SubscriptionBoundSessionConsumerSuite) TestHandleHappyPath() {
	userID := uuid.New()
	raw, _ := json.Marshal(map[string]any{
		"event_id": uuid.New().String(),
		"user_id":  userID.String(),
	})
	env := outbox.Envelope{ID: uuid.New().String(), Payload: raw}

	uc := &fakeStartBudgetUC{result: usecases.StartBudgetConfigurationResult{Outcome: usecases.StartBudgetOutcomeStarted}}
	c := consumers.NewSubscriptionBoundSessionConsumer(uc, noop.NewProvider())

	err := c.Handle(context.Background(), subscriptionBoundStubEvent{payload: env})
	s.Require().NoError(err)
	s.True(uc.called)
	s.Equal(userID, uc.input.UserID)
}

func (s *SubscriptionBoundSessionConsumerSuite) TestHandleRejectsUnexpectedPayloadType() {
	uc := &fakeStartBudgetUC{}
	c := consumers.NewSubscriptionBoundSessionConsumer(uc, noop.NewProvider())

	err := c.Handle(context.Background(), subscriptionBoundStubEvent{payload: "not-envelope"})
	s.Require().Error(err)
	s.ErrorContains(err, "unexpected payload type")
	s.False(uc.called)
}

func (s *SubscriptionBoundSessionConsumerSuite) TestHandleRejectsInvalidUserID() {
	raw, _ := json.Marshal(map[string]any{"user_id": "not-a-uuid"})
	env := outbox.Envelope{Payload: raw}
	uc := &fakeStartBudgetUC{}
	c := consumers.NewSubscriptionBoundSessionConsumer(uc, noop.NewProvider())

	err := c.Handle(context.Background(), subscriptionBoundStubEvent{payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "user_id invalido")
	s.False(uc.called)
}

func (s *SubscriptionBoundSessionConsumerSuite) TestHandlePropagatesUseCaseError() {
	userID := uuid.New()
	raw, _ := json.Marshal(map[string]any{"user_id": userID.String()})
	env := outbox.Envelope{Payload: raw}
	uc := &fakeStartBudgetUC{err: errors.New("downstream error")}
	c := consumers.NewSubscriptionBoundSessionConsumer(uc, noop.NewProvider())

	err := c.Handle(context.Background(), subscriptionBoundStubEvent{payload: env})
	s.Require().Error(err)
	s.ErrorContains(err, "start budget")
}
