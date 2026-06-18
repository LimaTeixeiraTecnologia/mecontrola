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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type ProjectSubscriptionEventIntegrationSuite struct {
	suite.Suite
	ctx  context.Context
	db   *sqlx.DB
	o11y *noop.Provider
}

func TestProjectSubscriptionEventIntegration(t *testing.T) {
	suite.Run(t, new(ProjectSubscriptionEventIntegrationSuite))
}

func (s *ProjectSubscriptionEventIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *ProjectSubscriptionEventIntegrationSuite) seedUser(userID string) {
	phone := "+5511" + userID[:8]
	_, err := s.db.ExecContext(s.ctx,
		`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, phone,
	)
	s.Require().NoError(err)
}

func (s *ProjectSubscriptionEventIntegrationSuite) SetupSuite() {
	db, _ := testcontainer.Postgres(s.T())
	s.db = db
	s.o11y = noop.NewProvider()
}

func (s *ProjectSubscriptionEventIntegrationSuite) newSUT(reader interfaces.SubscriptionProjectionReader) *usecases.ProjectSubscriptionEvent {
	factory := repositories.NewRepositoryFactory(s.o11y)
	repo := factory.EntitlementRepository(s.db)
	return usecases.NewProjectSubscriptionEvent(repo, reader, s.o11y)
}

func (s *ProjectSubscriptionEventIntegrationSuite) buildPayload(subscriptionID string) json.RawMessage {
	m := map[string]string{"subscription_id": subscriptionID}
	raw, err := json.Marshal(m)
	s.Require().NoError(err)
	return json.RawMessage(raw)
}

func (s *ProjectSubscriptionEventIntegrationSuite) findEntitlementStatus(userID string) string {
	var status string
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT status FROM identity_entitlements WHERE user_id = $1`,
		userID,
	).Scan(&status)
	s.Require().NoError(err)
	return status
}

func (s *ProjectSubscriptionEventIntegrationSuite) countEntitlementsByUserID(userID string) int {
	var total int
	err := s.db.QueryRowContext(
		s.ctx,
		`SELECT COUNT(*) FROM identity_entitlements WHERE user_id = $1`,
		userID,
	).Scan(&total)
	s.Require().NoError(err)
	return total
}

func (s *ProjectSubscriptionEventIntegrationSuite) TestProjectSubscriptionEventActivated() {
	userID := uuid.New().String()
	subscriptionID := uuid.New().String()
	s.seedUser(userID)

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Status:         "ACTIVE",
		PeriodEnd:      time.Now().UTC().Add(30 * 24 * time.Hour),
		OccurredAt:     time.Now().UTC(),
	}

	reader := mocks.NewSubscriptionProjectionReader(s.T())
	reader.EXPECT().
		FindCurrentBySubscriptionID(s.ctx, subscriptionID).
		Return(projection, nil)

	sut := s.newSUT(reader)
	err := sut.Execute(s.ctx, input.ProjectSubscriptionEvent{
		EventType: "billing.subscription.activated",
		Payload:   s.buildPayload(subscriptionID),
	})

	s.Require().NoError(err)
	s.Equal("ACTIVE", s.findEntitlementStatus(userID))
}

func (s *ProjectSubscriptionEventIntegrationSuite) TestProjectSubscriptionEventIdempotency() {
	userID := uuid.New().String()
	subscriptionID := uuid.New().String()
	s.seedUser(userID)

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Status:         "ACTIVE",
		PeriodEnd:      time.Now().UTC().Add(30 * 24 * time.Hour),
		OccurredAt:     time.Now().UTC(),
	}

	reader := mocks.NewSubscriptionProjectionReader(s.T())
	reader.EXPECT().
		FindCurrentBySubscriptionID(s.ctx, subscriptionID).
		Return(projection, nil).
		Times(2)

	sut := s.newSUT(reader)

	payload := s.buildPayload(subscriptionID)

	err := sut.Execute(s.ctx, input.ProjectSubscriptionEvent{
		EventType: "billing.subscription.activated",
		Payload:   payload,
	})
	s.Require().NoError(err)

	err2 := sut.Execute(s.ctx, input.ProjectSubscriptionEvent{
		EventType: "billing.subscription.activated",
		Payload:   payload,
	})
	s.Require().NoError(err2)

	s.Equal(1, s.countEntitlementsByUserID(userID))
	s.Equal("ACTIVE", s.findEntitlementStatus(userID))
}

func (s *ProjectSubscriptionEventIntegrationSuite) TestProjectSubscriptionEventCanceled() {
	userID := uuid.New().String()
	subscriptionID := uuid.New().String()
	s.seedUser(userID)

	projection := interfaces.SubscriptionProjectionRecord{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Status:         "EXPIRED",
		PeriodEnd:      time.Now().UTC(),
		OccurredAt:     time.Now().UTC(),
	}

	reader := mocks.NewSubscriptionProjectionReader(s.T())
	reader.EXPECT().
		FindCurrentBySubscriptionID(s.ctx, subscriptionID).
		Return(projection, nil)

	sut := s.newSUT(reader)
	err := sut.Execute(s.ctx, input.ProjectSubscriptionEvent{
		EventType: "billing.subscription.canceled",
		Payload:   s.buildPayload(subscriptionID),
	})

	s.Require().NoError(err)
	s.Equal("EXPIRED", s.findEntitlementStatus(userID))
}
