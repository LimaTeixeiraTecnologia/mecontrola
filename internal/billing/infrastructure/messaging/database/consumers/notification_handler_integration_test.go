//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/billing/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type stubNotificationSender struct {
	calls []interfaces.NotificationPayload
}

func (s *stubNotificationSender) NotifyTransition(_ context.Context, p interfaces.NotificationPayload) error {
	s.calls = append(s.calls, p)
	return nil
}

type wrongPayloadEvent struct{}

func (e *wrongPayloadEvent) GetEventType() string { return producers.EventTypeSubscriptionPastDue }
func (e *wrongPayloadEvent) GetPayload() any      { return "not-an-envelope" }

type envelopeEvent struct {
	eventType string
	envelope  outbox.Envelope
}

func (e *envelopeEvent) GetEventType() string { return e.eventType }
func (e *envelopeEvent) GetPayload() any      { return e.envelope }

func countPublishedOutboxByType(t *testing.T, db database.DBTX, eventType string) int {
	t.Helper()
	ctx := context.Background()
	var count int
	row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbox_events WHERE event_type = $1`, eventType)
	if err := row.Scan(&count); err != nil {
		t.Fatalf("countPublishedOutboxByType: %v", err)
	}
	return count
}

type NotificationHandlerIntegSuite struct {
	suite.Suite
	db      *sqlx.DB
	storage outbox.OutboxRepository
}

func TestNotificationHandlerIntegSuite(t *testing.T) {
	suite.Run(t, new(NotificationHandlerIntegSuite))
}

func (s *NotificationHandlerIntegSuite) SetupTest() {}

func (s *NotificationHandlerIntegSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.storage = outbox.NewPostgresStorage(db)
}

func (s *NotificationHandlerIntegSuite) buildHandler(sender *stubNotificationSender) *consumers.NotificationHandler {
	o11y := noop.NewProvider()
	uc := usecases.NewSendSubscriptionNotification(sender, o11y)
	return consumers.NewNotificationHandler(uc, producers.EventTypeSubscriptionPastDue, o11y)
}

func (s *NotificationHandlerIntegSuite) buildPastDueEnvelope(eventID string, subscriptionID string) outbox.Envelope {
	payload, err := json.Marshal(map[string]string{"subscription_id": subscriptionID})
	s.Require().NoError(err)
	return outbox.Envelope{
		ID:         eventID,
		EventType:  producers.EventTypeSubscriptionPastDue,
		OccurredAt: time.Now().UTC(),
		Metadata:   map[string]string{},
		Payload:    json.RawMessage(payload),
	}
}

func (s *NotificationHandlerIntegSuite) TestHandle_PastDueEvent_CallsNotificationSender() {
	scenarios := []struct {
		name           string
		subscriptionID string
	}{
		{name: "processa evento past_due e chama sender uma vez", subscriptionID: uuid.NewString()},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			sender := &stubNotificationSender{}
			handler := s.buildHandler(sender)

			eventID := uuid.NewString()
			envelope := s.buildPastDueEnvelope(eventID, scenario.subscriptionID)
			evt := &envelopeEvent{eventType: producers.EventTypeSubscriptionPastDue, envelope: envelope}

			err := handler.Handle(ctx, evt)
			s.Require().NoError(err)
			s.Len(sender.calls, 1)
		})
	}
}

func (s *NotificationHandlerIntegSuite) TestHandle_Idempotency_OutboxStorageDeduplicates() {
	scenarios := []struct {
		name string
	}{
		{name: "inserir mesmo event_id duas vezes mantém uma única linha no outbox"},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()

			eventID := uuid.NewString()
			subscriptionID := uuid.NewString()
			payload, err := json.Marshal(map[string]string{"subscription_id": subscriptionID})
			s.Require().NoError(err)

			evt, err := outbox.NewEvent(outbox.EventInput{
				ID:            eventID,
				Type:          producers.EventTypeSubscriptionPastDue,
				AggregateType: "subscription",
				AggregateID:   uuid.NewString(),
				Payload:       payload,
				OccurredAt:    time.Now().UTC(),
			})
			s.Require().NoError(err)

			beforeCount := countPublishedOutboxByType(s.T(), s.db, producers.EventTypeSubscriptionPastDue)

			s.Require().NoError(s.storage.Insert(ctx, evt, 5))
			s.Require().NoError(s.storage.Insert(ctx, evt, 5))

			afterCount := countPublishedOutboxByType(s.T(), s.db, producers.EventTypeSubscriptionPastDue)
			s.Equal(beforeCount+1, afterCount, "reprocessar mesmo event_id não deve duplicar linha no outbox")

			rows, claimErr := s.storage.ClaimBatch(ctx, "test-worker", 10)
			s.Require().NoError(claimErr)

			sender := &stubNotificationSender{}
			handler := s.buildHandler(sender)

			var handled int
			for _, row := range rows {
				if row.ID != eventID {
					continue
				}
				envelope := outbox.Pack(row)
				e := &envelopeEvent{eventType: row.Type, envelope: envelope}
				s.Require().NoError(handler.Handle(ctx, e))
				handled++
			}

			s.Equal(1, handled, "deve haver exatamente uma linha para o event_id duplicado")
			s.Len(sender.calls, 1, "sender deve ser chamado uma única vez para o evento deduplicado")

			s.T().Cleanup(func() {
				_, _ = s.db.ExecContext(ctx, `DELETE FROM outbox_events WHERE id = $1`, eventID)
			})
		})
	}
}

func (s *NotificationHandlerIntegSuite) TestHandle_UnknownPayloadType_ReturnsNilWithoutCallingSender() {
	scenarios := []struct {
		name string
		evt  events.Event
	}{
		{
			name: "payload não é outbox.Envelope → handler retorna nil sem chamar sender",
			evt:  &wrongPayloadEvent{},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ctx := context.Background()
			sender := &stubNotificationSender{}
			handler := s.buildHandler(sender)

			err := handler.Handle(ctx, scenario.evt)
			s.Require().NoError(err)
			s.Empty(sender.calls)
		})
	}
}
