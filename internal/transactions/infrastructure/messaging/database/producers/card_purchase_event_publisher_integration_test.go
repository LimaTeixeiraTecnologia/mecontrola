//go:build integration

package producers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/producers"
)

type CardPurchaseEventPublisherIntegrationSuite struct {
	suite.Suite
	publisher *producers.CardPurchaseEventPublisher
}

func TestCardPurchaseEventPublisherIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CardPurchaseEventPublisherIntegrationSuite))
}

func (s *CardPurchaseEventPublisherIntegrationSuite) SetupSuite() {
	mgr, _ := testcontainer.Postgres(s.T())
	_ = mgr

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	cfg := configs.OutboxConfig{RetryMaxAttempts: 3}
	s.publisher = producers.NewCardPurchaseEventPublisher(outboxFactory, cfg)
}

func (s *CardPurchaseEventPublisherIntegrationSuite) TestPublishCreated_SingleEventWithRefMonths() {
	mgr, _ := testcontainer.Postgres(s.T())
	db := mgr.DBTX(context.Background())

	rm1, _ := valueobjects.NewRefMonth("2024-01")
	rm2, _ := valueobjects.NewRefMonth("2024-02")
	rm3, _ := valueobjects.NewRefMonth("2024-03")

	evt := entities.CardPurchaseCreated{
		EventID:           uuid.New(),
		AggregateID:       uuid.New(),
		UserID:            uuid.New(),
		OccurredAt:        time.Now().UTC(),
		CardID:            uuid.New(),
		TotalAmountCents:  3000,
		InstallmentsTotal: 3,
		RefMonthsAffected: []valueobjects.RefMonth{rm1, rm2, rm3},
	}

	err := s.publisher.PublishCreated(context.Background(), db, evt)
	s.Require().NoError(err)
}

func (s *CardPurchaseEventPublisherIntegrationSuite) TestPublishUpdated_WithInvoiceDeltas() {
	mgr, _ := testcontainer.Postgres(s.T())
	db := mgr.DBTX(context.Background())

	rm1, _ := valueobjects.NewRefMonth("2024-01")

	evt := entities.CardPurchaseUpdated{
		EventID:           uuid.New(),
		AggregateID:       uuid.New(),
		UserID:            uuid.New(),
		OccurredAt:        time.Now().UTC(),
		CardID:            uuid.New(),
		TotalAmountCents:  1000,
		InstallmentsTotal: 1,
		RefMonthsAffected: []valueobjects.RefMonth{rm1},
		InvoiceDeltas:     map[string]int64{"2024-01": -500},
	}

	err := s.publisher.PublishUpdated(context.Background(), db, evt)
	s.Require().NoError(err)
}

func (s *CardPurchaseEventPublisherIntegrationSuite) TestPublishDeleted_WithNegativeDeltas() {
	mgr, _ := testcontainer.Postgres(s.T())
	db := mgr.DBTX(context.Background())

	rm1, _ := valueobjects.NewRefMonth("2024-05")

	evt := entities.CardPurchaseDeleted{
		EventID:           uuid.New(),
		AggregateID:       uuid.New(),
		UserID:            uuid.New(),
		OccurredAt:        time.Now().UTC(),
		CardID:            uuid.New(),
		RefMonthsAffected: []valueobjects.RefMonth{rm1},
		InvoiceDeltas:     map[string]int64{"2024-05": -2000},
	}

	err := s.publisher.PublishDeleted(context.Background(), db, evt)
	s.Require().NoError(err)
}
