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

type SubscriptionBoundProjectorIntegrationSuite struct {
	suite.Suite
	ctx context.Context
	db  *sqlx.DB
}

func TestSubscriptionBoundProjectorIntegration(t *testing.T) {
	suite.Run(t, new(SubscriptionBoundProjectorIntegrationSuite))
}

func (s *SubscriptionBoundProjectorIntegrationSuite) SetupSuite() {
	s.db, _ = testcontainer.Postgres(s.T())
}

func (s *SubscriptionBoundProjectorIntegrationSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *SubscriptionBoundProjectorIntegrationSuite) newSUT(reader interfaces.SubscriptionProjectionReader) *consumers.SubscriptionBoundProjector {
	o11y := noop.NewProvider()
	factory := repositories.NewRepositoryFactory(o11y)
	entitlementRepo := factory.EntitlementRepository(s.db)
	projectSubscriptionEventUC := usecases.NewProjectSubscriptionEvent(entitlementRepo, reader, o11y)
	return consumers.NewSubscriptionBoundProjector(projectSubscriptionEventUC, o11y)
}

func (s *SubscriptionBoundProjectorIntegrationSuite) makeBoundEvent(subscriptionID, funnelToken string) *fakeEvent {
	eventID := uuid.New()
	payload, err := json.Marshal(map[string]any{
		"subscription_id": subscriptionID,
		"funnel_token":    funnelToken,
		"event_id":        eventID.String(),
	})
	s.Require().NoError(err)

	return &fakeEvent{
		eventType: "onboarding.subscription_bound",
		payload: outbox.Envelope{
			ID:        eventID.String(),
			EventType: "onboarding.subscription_bound",
			Payload:   payload,
		},
	}
}

func (s *SubscriptionBoundProjectorIntegrationSuite) buildPendingProjection(subscriptionID, funnelToken string) interfaces.SubscriptionProjectionRecord {
	return interfaces.SubscriptionProjectionRecord{
		SubscriptionID: subscriptionID,
		FunnelToken:    funnelToken,
		UserID:         "",
		Status:         "PENDING",
		PeriodEnd:      time.Now().UTC().Add(30 * 24 * time.Hour),
		OccurredAt:     time.Now().UTC(),
	}
}

func (s *SubscriptionBoundProjectorIntegrationSuite) countPending(subscriptionID string) int {
	var n int
	err := s.db.QueryRowContext(s.ctx,
		`SELECT COUNT(*) FROM identity_entitlements_pending WHERE subscription_id = $1`,
		subscriptionID,
	).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *SubscriptionBoundProjectorIntegrationSuite) TestBoundInsertsRow() {
	s.Run("onboarding.subscription_bound insere linha em identity_entitlements_pending", func() {
		subID := uuid.New().String()
		funnelToken := "tk-" + uuid.New().String()

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildPendingProjection(subID, funnelToken), nil).Once()

		sut := s.newSUT(reader)
		evt := s.makeBoundEvent(subID, funnelToken)

		err := sut.Handle(s.ctx, evt)
		s.Require().NoError(err)

		s.Equal(1, s.countPending(subID))
	})
}

func (s *SubscriptionBoundProjectorIntegrationSuite) TestBoundIdempotence() {
	s.Run("reprocessar mesmo subscription_id mantem COUNT igual a 1 (upsert idempotente)", func() {
		subID := uuid.New().String()
		funnelToken := "tk-idem-" + uuid.New().String()

		reader := interfacemocks.NewSubscriptionProjectionReader(s.T())
		reader.EXPECT().FindCurrentBySubscriptionID(mock.Anything, subID).
			Return(s.buildPendingProjection(subID, funnelToken), nil).Times(2)

		sut := s.newSUT(reader)

		evt1 := s.makeBoundEvent(subID, funnelToken)
		err := sut.Handle(s.ctx, evt1)
		s.Require().NoError(err)

		evt2 := s.makeBoundEvent(subID, funnelToken)
		err = sut.Handle(s.ctx, evt2)
		s.Require().NoError(err)

		s.Equal(1, s.countPending(subID))
	})
}
