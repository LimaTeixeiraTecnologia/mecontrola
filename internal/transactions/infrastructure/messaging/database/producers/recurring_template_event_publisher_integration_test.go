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
	publisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, outboxCfg())

	evt := entities.RecurringTemplateCreated{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}

	s.Require().NoError(publisher.PublishCreated(ctx, db, evt))
}

func (s *RecurringTemplateEventPublisherSuite) TestPublishUpdated_Persists() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, outboxCfg())

	evt := entities.RecurringTemplateUpdated{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}

	s.Require().NoError(publisher.PublishUpdated(ctx, db, evt))
}

func (s *RecurringTemplateEventPublisherSuite) TestPublishDeleted_Persists() {
	mgr, _ := testcontainer.Postgres(s.T())
	ctx := context.Background()
	db := mgr.DBTX(ctx)

	outboxFactory := outbox.NewRepositoryFactory(noop.NewProvider())
	publisher := producers.NewRecurringTemplateEventPublisher(outboxFactory, outboxCfg())

	evt := entities.RecurringTemplateDeleted{
		EventID:     uuid.New(),
		AggregateID: uuid.New(),
		UserID:      uuid.New(),
		OccurredAt:  time.Now().UTC(),
	}

	s.Require().NoError(publisher.PublishDeleted(ctx, db, evt))
}
