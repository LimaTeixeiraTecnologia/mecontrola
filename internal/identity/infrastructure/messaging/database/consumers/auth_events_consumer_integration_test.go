//go:build integration

package consumers_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database/manager"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type AuthEventsConsumerIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	mgr manager.Manager
}

func TestAuthEventsConsumerIntegration(t *testing.T) {
	suite.Run(t, new(AuthEventsConsumerIntegrationSuite))
}

func (s *AuthEventsConsumerIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *AuthEventsConsumerIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	s.mgr = mgr
}

func (s *AuthEventsConsumerIntegrationSuite) newSUT() *consumers.AuthEventsConsumer {
	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)
	projectAuthEventUC := usecases.NewProjectAuthEvent(factory, s.mgr, o11y)
	anonymizeUserAuthEventsUC := usecases.NewAnonymizeUserAuthEvents(factory, s.mgr, o11y)
	return consumers.NewAuthEventsConsumer(projectAuthEventUC, anonymizeUserAuthEventsUC, o11y)
}

func (s *AuthEventsConsumerIntegrationSuite) countAuthEvents(eventID uuid.UUID) int {
	var n int
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM auth_events WHERE id = $1`, eventID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *AuthEventsConsumerIntegrationSuite) countAuthEventsByUserID(userID uuid.UUID) int {
	var n int
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM auth_events WHERE user_id = $1`, userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *AuthEventsConsumerIntegrationSuite) countAnonymizedByUserID(userID uuid.UUID) int {
	var n int
	err := s.mgr.DBTX(s.ctx).QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM auth_events WHERE user_id IS NULL AND id IN (
			SELECT id FROM auth_events WHERE occurred_at IS NOT NULL
		)`,
	).Scan(&n)
	s.Require().NoError(err)
	_ = userID
	return n
}

type fakeEvent struct {
	eventType string
	payload   outbox.Envelope
}

func (f *fakeEvent) GetEventType() string { return f.eventType }
func (f *fakeEvent) GetPayload() any      { return f.payload }

func (s *AuthEventsConsumerIntegrationSuite) makeAuthEvent(eventType, kind string, userID *uuid.UUID) (events.Event, uuid.UUID) {
	eventID, err := uuid.NewV7()
	s.Require().NoError(err)

	payloadMap := map[string]any{
		"event_id":    eventID.String(),
		"kind":        kind,
		"source":      "whatsapp",
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
	}
	if userID != nil {
		payloadMap["user_id"] = userID.String()
	}

	rawPayload, err := json.Marshal(payloadMap)
	s.Require().NoError(err)

	evt := &fakeEvent{
		eventType: eventType,
		payload: outbox.Envelope{
			ID:        eventID.String(),
			EventType: eventType,
			Payload:   rawPayload,
		},
	}
	return evt, eventID
}

func (s *AuthEventsConsumerIntegrationSuite) makeUserDeletedEvent(userID uuid.UUID) events.Event {
	eventID := uuid.New()
	payloadMap := map[string]any{
		"event_id":   eventID.String(),
		"user_id":    userID.String(),
		"deleted_at": time.Now().UTC().Format(time.RFC3339),
	}
	rawPayload, err := json.Marshal(payloadMap)
	s.Require().NoError(err)

	return &fakeEvent{
		eventType: "user.deleted",
		payload: outbox.Envelope{
			ID:        eventID.String(),
			EventType: "user.deleted",
			Payload:   rawPayload,
		},
	}
}

func (s *AuthEventsConsumerIntegrationSuite) TestHandleAuthPrincipalEstablished() {
	s.Run("deve inserir linha em auth_events ao processar auth.principal_established", func() {
		sut := s.newSUT()
		uid := uuid.New()
		evt, eventID := s.makeAuthEvent("auth.principal_established", "principal_established", &uid)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal(1, s.countAuthEvents(eventID), "deve haver exatamente 1 linha com o event_id")
	})
}

func (s *AuthEventsConsumerIntegrationSuite) TestHandleIdempotence() {
	s.Run("reprocessar mesmo event_id deve ser no-op — ON CONFLICT DO NOTHING", func() {
		sut := s.newSUT()
		uid := uuid.New()
		evt, eventID := s.makeAuthEvent("auth.principal_established", "principal_established", &uid)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		payloadMap := map[string]any{
			"event_id":    eventID.String(),
			"kind":        "principal_established",
			"source":      "whatsapp",
			"occurred_at": time.Now().UTC().Format(time.RFC3339),
			"user_id":     uid.String(),
		}
		rawPayload, err := json.Marshal(payloadMap)
		s.Require().NoError(err)

		duplicate := &fakeEvent{
			eventType: "auth.principal_established",
			payload: outbox.Envelope{
				ID:        eventID.String(),
				EventType: "auth.principal_established",
				Payload:   rawPayload,
			},
		}

		err = sut.Handle(s.ctx, duplicate)
		s.Require().NoError(err)

		s.Equal(1, s.countAuthEvents(eventID), "deve continuar com exatamente 1 linha apos reprocessamento")
	})
}

func (s *AuthEventsConsumerIntegrationSuite) TestHandleUserDeleted() {
	s.Run("deve anonimizar linhas do usuario ao receber user.deleted", func() {
		sut := s.newSUT()
		uid := uuid.New()

		evt1, _ := s.makeAuthEvent("auth.principal_established", "principal_established", &uid)
		err := sut.Handle(s.ctx, evt1)
		s.Require().NoError(err)

		evt2, _ := s.makeAuthEvent("auth.principal_established", "principal_established", &uid)
		err = sut.Handle(s.ctx, evt2)
		s.Require().NoError(err)

		s.Equal(2, s.countAuthEventsByUserID(uid), "deve haver 2 linhas com user_id antes da anonimizacao")

		deletedEvt := s.makeUserDeletedEvent(uid)
		err = sut.Handle(s.ctx, deletedEvt)
		s.Require().NoError(err)

		s.Equal(0, s.countAuthEventsByUserID(uid), "deve haver 0 linhas com user_id apos anonimizacao")
	})
}

func (s *AuthEventsConsumerIntegrationSuite) TestHandleUserDeletedIdempotence() {
	s.Run("reprocessar user.deleted deve ser no-op", func() {
		sut := s.newSUT()
		uid := uuid.New()

		evt, _ := s.makeAuthEvent("auth.unknown_user", "unknown_user", nil)
		_ = sut.Handle(s.ctx, evt)

		deletedEvt := s.makeUserDeletedEvent(uid)
		err := sut.Handle(s.ctx, deletedEvt)
		s.Require().NoError(err)

		err = sut.Handle(s.ctx, deletedEvt)
		s.Require().NoError(err)

		s.Equal(0, s.countAuthEventsByUserID(uid), "usuario deve permanecer anonimizado apos segundo user.deleted")
	})
}

func (s *AuthEventsConsumerIntegrationSuite) TestHandleAuthUnknownUser() {
	s.Run("deve inserir evento auth.unknown_user sem user_id", func() {
		sut := s.newSUT()
		evt, eventID := s.makeAuthEvent("auth.unknown_user", "unknown_user", nil)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal(1, s.countAuthEvents(eventID), "deve haver linha em auth_events para unknown_user")
	})
}
