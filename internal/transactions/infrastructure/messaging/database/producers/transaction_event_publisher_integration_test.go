//go:build integration

package producers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/database"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
	producers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/producers"
)

type TransactionEventPublisherSuite struct {
	suite.Suite
}

func TestTransactionEventPublisherSuite(t *testing.T) {
	suite.Run(t, new(TransactionEventPublisherSuite))
}

func outboxCfg() configs.OutboxConfig {
	return configs.OutboxConfig{RetryMaxAttempts: 3}
}

func (s *TransactionEventPublisherSuite) TestPublishCreated_SameTX_Persists() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewTransactionEventPublisher(outboxFactory, outboxCfg())

	rm, _ := valueobjects.NewRefMonth("2026-06")
	evt := entities.TransactionCreated{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
		RefMonth:    rm,
	}

	s.Require().NoError(publisher.PublishCreated(ctx, db, evt))
}

func (s *TransactionEventPublisherSuite) TestPublishCreated_RollbackDiscardsEvent() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewTransactionEventPublisher(outboxFactory, outboxCfg())

	tx, err := mgr.BeginTx(ctx, database.TxOptions{})
	s.Require().NoError(err)

	rm, _ := valueobjects.NewRefMonth("2026-06")
	aggregateID := uuid.New()
	evt := entities.TransactionCreated{
		EventID:     uuid.New(),
		AggregateID: aggregateID,
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
		RefMonth:    rm,
	}

	s.Require().NoError(publisher.PublishCreated(ctx, tx, evt))
	s.Require().NoError(tx.Rollback(ctx))

	cleanDB := mgr.DBTX(ctx)
	storage := outbox.NewPostgresStorage(cleanDB)
	rows, claimErr := storage.ClaimBatch(ctx, "test-verifier", 100)
	s.Require().NoError(claimErr)

	for _, row := range rows {
		s.NotEqual(aggregateID.String(), row.AggregateID, "evento rollado nao deve aparecer no outbox")
	}
}

func (s *TransactionEventPublisherSuite) TestPublishUpdated_Success() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewTransactionEventPublisher(outboxFactory, outboxCfg())

	rm, _ := valueobjects.NewRefMonth("2026-06")
	evt := entities.TransactionUpdated{
		EventID:           uuid.New(),
		AggregateID:       uuid.New(),
		UserID:            uuid.New(),
		OccurredAt:        time.Now().UTC(),
		RefMonth:          rm,
		RefMonthsAffected: []valueobjects.RefMonth{rm},
	}

	s.Require().NoError(publisher.PublishUpdated(ctx, db, evt))
}

func (s *TransactionEventPublisherSuite) TestPublishDeleted_Success() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewTransactionEventPublisher(outboxFactory, outboxCfg())

	rm, _ := valueobjects.NewRefMonth("2026-06")
	evt := entities.TransactionDeleted{
		EventID:           uuid.New(),
		AggregateID:       uuid.New(),
		UserID:            uuid.New(),
		OccurredAt:        time.Now().UTC(),
		RefMonth:          rm,
		RefMonthsAffected: []valueobjects.RefMonth{rm},
	}

	s.Require().NoError(publisher.PublishDeleted(ctx, db, evt))
}
