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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	interfacemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/messaging/database/consumers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
)

type SubscriptionEventProjectorIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestSubscriptionEventProjectorIntegration(t *testing.T) {
	suite.Run(t, new(SubscriptionEventProjectorIntegrationSuite))
}

func (s *SubscriptionEventProjectorIntegrationSuite) SetupSuite() {
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *SubscriptionEventProjectorIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubscriptionEventProjectorIntegrationSuite) newSUT(reader interfaces.SubscriptionProjectionReader) *consumers.SubscriptionEventProjector {
	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)
	entitlementRepo := factory.EntitlementRepository(s.db)
	projectSubscriptionEventUC := usecases.NewProjectSubscriptionEvent(entitlementRepo, reader, o11y)
	return consumers.NewSubscriptionEventProjector(projectSubscriptionEventUC, o11y)
}

func (s *SubscriptionEventProjectorIntegrationSuite) makeSubscriptionEvent(eventType, subscriptionID string) *fakeEvent {
	eventID := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"subscription_id": subscriptionID,
		"event_id":        eventID.String(),
	})
	s.Require().NoError(err)

	return &fakeEvent{
		eventType: eventType,
		payload: outbox.Envelope{
			ID:        eventID.String(),
			EventType: eventType,
			Payload:   payload,
		},
	}
}

func (s *SubscriptionEventProjectorIntegrationSuite) queryStatus(userID string) string {
	var status string
	err := s.db.QueryRowContext(s.ctx,
		`SELECT status FROM identity_entitlements WHERE user_id = $1`,
		userID,
	).Scan(&status)
	s.Require().NoError(err)
	return status
}

func (s *SubscriptionEventProjectorIntegrationSuite) countEntitlements(userID string) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM identity_entitlements WHERE user_id = $1`,
		userID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *SubscriptionEventProjectorIntegrationSuite) seedUser(userID string) {
	phone := "+5511" + userID[:8]
	_, err := s.db.ExecContext(s.ctx,
		`INSERT INTO users (id, whatsapp_number, status, created_at, updated_at)
		 VALUES ($1, $2, 'ACTIVE', now(), now())`,
		userID, phone,
	)
	s.Require().NoError(err)
}

func (s *SubscriptionEventProjectorIntegrationSuite) buildCommittedProjection(userID, subscriptionID, status string) interfaces.SubscriptionProjectionRecord {
	return interfaces.SubscriptionProjectionRecord{
		SubscriptionID: subscriptionID,
		UserID:         userID,
		Status:         status,
		PeriodEnd:      time.Now().UTC().Add(30 * 24 * time.Hour),
		OccurredAt:     time.Now().UTC(),
	}
}

func (s *SubscriptionEventProjectorIntegrationSuite) TestActivated() {
	s.Run("billing.subscription.activated grava status ACTIVE em identity_entitlements", func() {
		userID := uuid.New().String()
		subID := uuid.New().String()
		s.seedUser(userID)

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildCommittedProjection(userID, subID, "ACTIVE"), nil).Once()

		sut := s.newSUT(reader)
		evt := s.makeSubscriptionEvent("billing.subscription.activated", subID)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal("ACTIVE", s.queryStatus(userID))
	})
}

func (s *SubscriptionEventProjectorIntegrationSuite) TestPastDue() {
	s.Run("billing.subscription.past_due grava status PAST_DUE em identity_entitlements", func() {
		userID := uuid.New().String()
		subID := uuid.New().String()
		s.seedUser(userID)

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildCommittedProjection(userID, subID, "PAST_DUE"), nil).Once()

		sut := s.newSUT(reader)
		evt := s.makeSubscriptionEvent("billing.subscription.past_due", subID)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal("PAST_DUE", s.queryStatus(userID))
	})
}

func (s *SubscriptionEventProjectorIntegrationSuite) TestCanceled() {
	s.Run("billing.subscription.canceled grava status EXPIRED em identity_entitlements", func() {
		userID := uuid.New().String()
		subID := uuid.New().String()
		s.seedUser(userID)

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildCommittedProjection(userID, subID, "EXPIRED"), nil).Once()

		sut := s.newSUT(reader)
		evt := s.makeSubscriptionEvent("billing.subscription.canceled", subID)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal("EXPIRED", s.queryStatus(userID))
	})
}

func (s *SubscriptionEventProjectorIntegrationSuite) TestRefunded() {
	s.Run("billing.subscription.refunded grava status REFUNDED em identity_entitlements", func() {
		userID := uuid.New().String()
		subID := uuid.New().String()
		s.seedUser(userID)

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildCommittedProjection(userID, subID, "REFUNDED"), nil).Once()

		sut := s.newSUT(reader)
		evt := s.makeSubscriptionEvent("billing.subscription.refunded", subID)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal("REFUNDED", s.queryStatus(userID))
	})
}

func (s *SubscriptionEventProjectorIntegrationSuite) TestIdempotence() {
	s.Run("reprocessar mesmo subscription_id mantem COUNT igual a 1 (upsert idempotente)", func() {
		userID := uuid.New().String()
		subID := uuid.New().String()
		s.seedUser(userID)

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildCommittedProjection(userID, subID, "ACTIVE"), nil).Times(2)

		sut := s.newSUT(reader)

		evt1 := s.makeSubscriptionEvent("billing.subscription.activated", subID)
		err := sut.Handle(s.ctx, evt1)
		s.Require().NoError(err)

		evt2 := s.makeSubscriptionEvent("billing.subscription.activated", subID)
		err = sut.Handle(s.ctx, evt2)
		s.Require().NoError(err)

		s.Equal(1, s.countEntitlements(userID))
	})
}
