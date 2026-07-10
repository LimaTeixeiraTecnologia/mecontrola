//go:build integration

package usecases_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ProjectAuthEventIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	db   *sqlx.DB
	o11y *noop.Provider
}

func TestProjectAuthEventIntegration(t *testing.T) {
	suite.Run(t, new(ProjectAuthEventIntegrationSuite))
}

func (s *ProjectAuthEventIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ProjectAuthEventIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
}

func (s *ProjectAuthEventIntegrationSuite) newSUT() *usecases.ProjectAuthEvent {
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.AuthEventsRepository(s.db)
	return usecases.NewProjectAuthEvent(repo, s.o11y)
}

func (s *ProjectAuthEventIntegrationSuite) buildPayload(eventID, userID, kind, source, reason string) []byte {
	m := map[string]interface{}{
		"event_id":    eventID,
		"kind":        kind,
		"source":      source,
		"occurred_at": time.Now().UTC().Format(time.RFC3339),
		"request_id":  "req-" + eventID,
		"client_ip":   "127.0.0.1",
	}
	if userID != "" {
		m["user_id"] = userID
	}
	if reason != "" {
		m["reason"] = reason
	}
	raw, err := json.Marshal(m)
	s.Require().NoError(err)
	return raw
}

func (s *ProjectAuthEventIntegrationSuite) buildPayloadWithResolvePath(eventID, userID, resolvePath string) []byte {
	m := map[string]interface{}{
		"event_id":     eventID,
		"kind":         "principal_established",
		"source":       "whatsapp",
		"occurred_at":  time.Now().UTC().Format(time.RFC3339),
		"user_id":      userID,
		"resolve_path": resolvePath,
	}
	raw, err := json.Marshal(m)
	s.Require().NoError(err)
	return raw
}

func (s *ProjectAuthEventIntegrationSuite) countAuthEventByID(eventID string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM auth_events WHERE id = $1`,
		eventID,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *ProjectAuthEventIntegrationSuite) TestProjectAuthEventPrincipalEstablished() {
	eventID := uuid.New().String()
	userID := uuid.New().String()
	payload := s.buildPayload(eventID, userID, "principal_established", "whatsapp", "")

	sut := s.newSUT()
	err := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.principal_established",
		Payload:   payload,
	})

	s.Require().NoError(err)
	s.Equal(1, s.countAuthEventByID(eventID))
}

func (s *ProjectAuthEventIntegrationSuite) TestProjectAuthEventIdempotency() {
	eventID := uuid.New().String()
	userID := uuid.New().String()
	payload := s.buildPayload(eventID, userID, "principal_established", "whatsapp", "")

	sut := s.newSUT()
	err := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.principal_established",
		Payload:   payload,
	})
	s.Require().NoError(err)

	err2 := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.principal_established",
		Payload:   payload,
	})
	s.Require().NoError(err2)
	s.Equal(1, s.countAuthEventByID(eventID))
}

func (s *ProjectAuthEventIntegrationSuite) TestProjectAuthEventUnknownUserNullUserID() {
	eventID := uuid.New().String()
	payload := s.buildPayload(eventID, "", "unknown_user", "whatsapp", "")

	sut := s.newSUT()
	err := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.unknown_user",
		Payload:   payload,
	})

	s.Require().NoError(err)
	s.Equal(1, s.countAuthEventByID(eventID))

	var userIDRaw *string
	queryErr := s.db.QueryRowContext(
		s.ctx,
		`SELECT user_id FROM auth_events WHERE id = $1`,
		eventID,
	).Scan(&userIDRaw)
	s.Require().NoError(queryErr)
	s.Nil(userIDRaw)
}

func (s *ProjectAuthEventIntegrationSuite) TestProjectAuthEventPersistsResolvePath() {
	eventID := uuid.New().String()
	userID := uuid.New().String()
	payload := s.buildPayloadWithResolvePath(eventID, userID, "legacy")

	sut := s.newSUT()
	err := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.principal_established",
		Payload:   payload,
	})
	s.Require().NoError(err)
	s.Equal(1, s.countAuthEventByID(eventID))

	var resolvePath *string
	queryErr := s.db.QueryRowContext(
		s.ctx,
		`SELECT resolve_path FROM auth_events WHERE id = $1`,
		eventID,
	).Scan(&resolvePath)
	s.Require().NoError(queryErr)
	s.Require().NotNil(resolvePath)
	s.Equal("legacy", *resolvePath)
}

func (s *ProjectAuthEventIntegrationSuite) TestProjectAuthEventNullResolvePath() {
	eventID := uuid.New().String()
	userID := uuid.New().String()
	payload := s.buildPayload(eventID, userID, "principal_established", "whatsapp", "")

	sut := s.newSUT()
	err := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.principal_established",
		Payload:   payload,
	})
	s.Require().NoError(err)

	var resolvePath *string
	queryErr := s.db.QueryRowContext(
		s.ctx,
		`SELECT resolve_path FROM auth_events WHERE id = $1`,
		eventID,
	).Scan(&resolvePath)
	s.Require().NoError(queryErr)
	s.Nil(resolvePath)
}

func (s *ProjectAuthEventIntegrationSuite) TestProjectAuthEventFailedWithReason() {
	eventID := uuid.New().String()
	payload := s.buildPayload(eventID, "", "failed", "whatsapp", "invalid_signature")

	sut := s.newSUT()
	err := sut.Execute(s.ctx, input.ProjectAuthEvent{
		EventType: "auth.failed",
		Payload:   payload,
	})

	s.Require().NoError(err)
	s.Equal(1, s.countAuthEventByID(eventID))

	var kind string
	queryErr := s.db.QueryRowContext(
		s.ctx,
		`SELECT kind FROM auth_events WHERE id = $1`,
		eventID,
	).Scan(&kind)
	s.Require().NoError(queryErr)
	s.Equal("failed", kind)
}
