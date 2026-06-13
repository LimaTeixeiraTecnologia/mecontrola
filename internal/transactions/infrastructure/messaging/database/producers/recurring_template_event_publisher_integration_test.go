//go:build integration

package producers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/testcontainer"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	producers "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/infrastructure/messaging/database/producers"
)

type RecurringTemplateEventPublisherSuite struct {
	suite.Suite
}

func TestRecurringTemplateEventPublisherSuite(t *testing.T) {
	suite.Run(t, new(RecurringTemplateEventPublisherSuite))
}

func (s *RecurringTemplateEventPublisherSuite) TestPublishCreated_Persists() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, outboxCfg(), noop.NewProvider())

	evt := entities.RecurringTemplateCreated{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}

	s.Require().NoError(publisher.PublishCreated(ctx, db, evt))

	storage := outbox.NewPostgresStorage(db)
	rows, err := storage.ClaimBatch(ctx, "test-rt-created", 100)
	s.Require().NoError(err)
	found := false
	for _, row := range rows {
		if row.AggregateID == evt.AggregateID.String() {
			found = true
			s.Equal(evt.UserID.String(), row.AggregateUserID)
		}
	}
	s.True(found, "evento recurring_template.created nao encontrado no outbox")
}

func (s *RecurringTemplateEventPublisherSuite) TestPublishUpdated_Persists() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, outboxCfg(), noop.NewProvider())

	evt := entities.RecurringTemplateUpdated{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}

	s.Require().NoError(publisher.PublishUpdated(ctx, db, evt))

	storage := outbox.NewPostgresStorage(db)
	rows, err := storage.ClaimBatch(ctx, "test-rt-updated", 100)
	s.Require().NoError(err)
	found := false
	for _, row := range rows {
		if row.AggregateID == evt.AggregateID.String() {
			found = true
			s.Equal(evt.UserID.String(), row.AggregateUserID)
		}
	}
	s.True(found, "evento recurring_template.updated nao encontrado no outbox")
}

func (s *RecurringTemplateEventPublisherSuite) TestPublishDeleted_Persists() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, outboxCfg(), noop.NewProvider())

	evt := entities.RecurringTemplateDeleted{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}

	s.Require().NoError(publisher.PublishDeleted(ctx, db, evt))

	storage := outbox.NewPostgresStorage(db)
	rows, err := storage.ClaimBatch(ctx, "test-rt-deleted", 100)
	s.Require().NoError(err)
	found := false
	for _, row := range rows {
		if row.AggregateID == evt.AggregateID.String() {
			found = true
			s.Equal(evt.UserID.String(), row.AggregateUserID)
		}
	}
	s.True(found, "evento recurring_template.deleted nao encontrado no outbox")
}
