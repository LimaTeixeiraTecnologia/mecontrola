package consumers_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type stubNotificationUC struct {
	called        bool
	capturedInput input.SendSubscriptionNotificationInput
	err           error
}

func (f *stubNotificationUC) Execute(_ context.Context, in input.SendSubscriptionNotificationInput) error {
	f.called = true
	f.capturedInput = in
	return f.err
}

type stubNotificationEvent struct {
	eventType string
	payload   any
}

func (e stubNotificationEvent) GetEventType() string { return e.eventType }
func (e stubNotificationEvent) GetPayload() any      { return e.payload }

type NotificationHandlerSuite struct {
	suite.Suite
}

func TestNotificationHandler(t *testing.T) {
	suite.Run(t, new(NotificationHandlerSuite))
}

func (s *NotificationHandlerSuite) buildEnvelope(subscriptionID string) outbox.Envelope {
	raw, err := json.Marshal(map[string]string{"subscription_id": subscriptionID})
	s.Require().NoError(err)
	return outbox.Envelope{
		ID:      "env-001",
		Payload: json.RawMessage(raw),
	}
}

func (s *NotificationHandlerSuite) TestHandle_PastDue_Success() {
	uc := &stubNotificationUC{}
	handler := consumers.NewNotificationHandler(uc, producers.EventTypeSubscriptionPastDue, noop.NewProvider())
	env := s.buildEnvelope("sub-past-due-001")

	err := handler.Handle(context.Background(), stubNotificationEvent{
		eventType: producers.EventTypeSubscriptionPastDue,
		payload:   env,
	})

	s.Require().NoError(err)
	s.True(uc.called)
	s.Equal(producers.EventTypeSubscriptionPastDue, uc.capturedInput.EventType)
	s.NotEmpty(uc.capturedInput.Payload)
}

func (s *NotificationHandlerSuite) TestHandle_Refunded_Success() {
	uc := &stubNotificationUC{}
	handler := consumers.NewNotificationHandler(uc, producers.EventTypeSubscriptionRefunded, noop.NewProvider())
	env := s.buildEnvelope("sub-refunded-001")

	err := handler.Handle(context.Background(), stubNotificationEvent{
		eventType: producers.EventTypeSubscriptionRefunded,
		payload:   env,
	})

	s.Require().NoError(err)
	s.True(uc.called)
	s.Equal(producers.EventTypeSubscriptionRefunded, uc.capturedInput.EventType)
	s.NotEmpty(uc.capturedInput.Payload)
}

func (s *NotificationHandlerSuite) TestHandle_ExpiredAfterGrace_Success() {
	uc := &stubNotificationUC{}
	handler := consumers.NewNotificationHandler(uc, producers.EventTypeSubscriptionExpired, noop.NewProvider())
	env := s.buildEnvelope("sub-expired-001")

	err := handler.Handle(context.Background(), stubNotificationEvent{
		eventType: producers.EventTypeSubscriptionExpired,
		payload:   env,
	})

	s.Require().NoError(err)
	s.True(uc.called)
	s.Equal(producers.EventTypeSubscriptionExpired, uc.capturedInput.EventType)
	s.NotEmpty(uc.capturedInput.Payload)
}

func (s *NotificationHandlerSuite) TestHandle_UsecaseError_IsPropagated() {
	sentinelErr := errors.New("usecase failure")
	uc := &stubNotificationUC{err: sentinelErr}
	handler := consumers.NewNotificationHandler(uc, producers.EventTypeSubscriptionPastDue, noop.NewProvider())
	env := s.buildEnvelope("sub-error-001")

	err := handler.Handle(context.Background(), stubNotificationEvent{
		eventType: producers.EventTypeSubscriptionPastDue,
		payload:   env,
	})

	s.Require().ErrorIs(err, sentinelErr)
	s.True(uc.called)
}

func (s *NotificationHandlerSuite) TestHandle_PayloadNotEnvelope_ReturnsNil() {
	uc := &stubNotificationUC{}
	handler := consumers.NewNotificationHandler(uc, producers.EventTypeSubscriptionPastDue, noop.NewProvider())

	err := handler.Handle(context.Background(), stubNotificationEvent{
		eventType: producers.EventTypeSubscriptionPastDue,
		payload:   "not-an-envelope",
	})

	s.Require().NoError(err)
	s.False(uc.called)
}
