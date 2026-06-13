package usecases_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type RecordGatewayAuthFailureSuite struct {
	suite.Suite
	ctx context.Context
}

func TestRecordGatewayAuthFailure(t *testing.T) {
	suite.Run(t, new(RecordGatewayAuthFailureSuite))
}

func (s *RecordGatewayAuthFailureSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_ValidReasons() {
	validReasons := []string{
		"gateway_missing_header",
		"gateway_invalid_timestamp",
		"gateway_stale_timestamp",
		"gateway_invalid_signature",
	}

	for _, reason := range validReasons {
		s.Run(reason, func() {
			publisher := outboxmocks.NewPublisher(s.T())
			publisher.EXPECT().Publish(mock.Anything, mock.MatchedBy(func(ev outbox.Event) bool {
				var payload map[string]any
				if err := json.Unmarshal(ev.Payload, &payload); err != nil {
					return false
				}
				return payload["reason"] == reason &&
					ev.Type == "auth.failed" &&
					ev.AggregateType == "auth_event"
			})).Return(nil).Once()

			sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
			err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
				Reason:      reason,
				RequestID:   "req-abc-123",
				ClientIPRaw: "192.168.1.1",
			})
			s.Require().NoError(err)
		})
	}
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_InvalidReason() {
	publisher := outboxmocks.NewPublisher(s.T())

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
		Reason: "unknown_reason",
	})
	s.Require().Error(err)
	s.ErrorIs(err, usecases.ErrInvalidGatewayReason)
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_PublishesRequestIDAndClientIP() {
	var capturedEvent outbox.Event

	publisher := outboxmocks.NewPublisher(s.T())
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ev outbox.Event) error {
		capturedEvent = ev
		return nil
	}).Once()

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
		Reason:      "gateway_invalid_signature",
		RequestID:   "req-xyz-001",
		ClientIPRaw: "10.0.0.1",
	})
	s.Require().NoError(err)

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(capturedEvent.Payload, &payload))
	s.Equal("req-xyz-001", payload["request_id"])
	s.Equal("10.0.0.1", payload["client_ip"])
	s.Equal("gateway_invalid_signature", payload["reason"])
	s.Equal("failed", payload["kind"])
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_UniqueEventIDsPerCall() {
	var eventIDs []string

	publisher := outboxmocks.NewPublisher(s.T())
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ev outbox.Event) error {
		var payload map[string]any
		if err := json.Unmarshal(ev.Payload, &payload); err != nil {
			return err
		}
		eventIDs = append(eventIDs, payload["event_id"].(string))
		return nil
	}).Times(2)

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	in := input.RecordGatewayAuthFailureInput{
		Reason: "gateway_missing_header",
	}

	s.Require().NoError(sut.Handle(s.ctx, in))
	s.Require().NoError(sut.Handle(s.ctx, in))

	s.Require().Len(eventIDs, 2)
	s.NotEqual(eventIDs[0], eventIDs[1], "event_id must be unique per call")
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_OutboxPublishError() {
	publisher := outboxmocks.NewPublisher(s.T())
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(errors.New("outbox down")).Once()

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
		Reason: "gateway_stale_timestamp",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "outbox down")
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_InvalidClientIP() {
	publisher := outboxmocks.NewPublisher(s.T())

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
		Reason:      "gateway_missing_header",
		ClientIPRaw: "not-a-valid-ip",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "parse client_ip")
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_WithoutRequestIDAndClientIP() {
	publisher := outboxmocks.NewPublisher(s.T())
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).Return(nil).Once()

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
		Reason: "gateway_invalid_signature",
	})
	s.Require().NoError(err)
}

func (s *RecordGatewayAuthFailureSuite) TestHandle_WithUserID() {
	var capturedEvent outbox.Event

	publisher := outboxmocks.NewPublisher(s.T())
	publisher.EXPECT().Publish(mock.Anything, mock.Anything).RunAndReturn(func(_ context.Context, ev outbox.Event) error {
		capturedEvent = ev
		return nil
	}).Once()

	sut := usecases.NewRecordGatewayAuthFailure(publisher, noop.NewProvider())
	err := sut.Handle(s.ctx, input.RecordGatewayAuthFailureInput{
		UserIDRaw: "a0a0a0a0-0000-0000-0000-000000000001",
		Reason:    "gateway_invalid_signature",
	})
	s.Require().NoError(err)

	var payload map[string]any
	s.Require().NoError(json.Unmarshal(capturedEvent.Payload, &payload))
	s.Equal("a0a0a0a0-0000-0000-0000-000000000001", payload["user_id"])
	s.Equal("a0a0a0a0-0000-0000-0000-000000000001", capturedEvent.AggregateID)
}
