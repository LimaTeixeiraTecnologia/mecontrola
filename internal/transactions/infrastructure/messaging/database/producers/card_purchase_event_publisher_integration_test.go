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
}

func TestCardPurchaseEventPublisherIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CardPurchaseEventPublisherIntegrationSuite))
}

func (s *CardPurchaseEventPublisherIntegrationSuite) TestPublishCreated_SingleEventWithRefMonths() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewCardPurchaseEventPublisher(outboxFactory, configs.OutboxConfig{RetryMaxAttempts: 3}, noop.NewProvider())

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

	s.Require().NoError(publisher.PublishCreated(ctx, db, evt))

	storage := outbox.NewPostgresStorage(db)
	rows, err := storage.ClaimBatch(ctx, "test-cp-created", 100)
	s.Require().NoError(err)
	found := false
	for _, row := range rows {
		if row.AggregateID == evt.AggregateID.String() {
			found = true
			s.Equal(evt.UserID.String(), row.AggregateUserID)
		}
	}
	s.True(found, "evento card_purchase.created nao encontrado no outbox")
}

func (s *CardPurchaseEventPublisherIntegrationSuite) TestPublishUpdated_WithInvoiceDeltas() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewCardPurchaseEventPublisher(outboxFactory, configs.OutboxConfig{RetryMaxAttempts: 3}, noop.NewProvider())

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

	s.Require().NoError(publisher.PublishUpdated(ctx, db, evt))

	storage := outbox.NewPostgresStorage(db)
	rows, err := storage.ClaimBatch(ctx, "test-cp-updated", 100)
	s.Require().NoError(err)
	found := false
	for _, row := range rows {
		if row.AggregateID == evt.AggregateID.String() {
			found = true
			s.Equal(evt.UserID.String(), row.AggregateUserID)
		}
	}
	s.True(found, "evento card_purchase.updated nao encontrado no outbox")
}

func (s *CardPurchaseEventPublisherIntegrationSuite) TestPublishDeleted_WithNegativeDeltas() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewCardPurchaseEventPublisher(outboxFactory, configs.OutboxConfig{RetryMaxAttempts: 3}, noop.NewProvider())

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

	s.Require().NoError(publisher.PublishDeleted(ctx, db, evt))

	storage := outbox.NewPostgresStorage(db)
	rows, err := storage.ClaimBatch(ctx, "test-cp-deleted", 100)
	s.Require().NoError(err)
	found := false
	for _, row := range rows {
		if row.AggregateID == evt.AggregateID.String() {
			found = true
			s.Equal(evt.UserID.String(), row.AggregateUserID)
		}
	}
	s.True(found, "evento card_purchase.deleted nao encontrado no outbox")
}
