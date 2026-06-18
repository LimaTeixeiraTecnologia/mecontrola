package producers_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/configs"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type BudgetActivatedPublisherSuite struct {
	suite.Suite
	storage     *outboxmocks.Storage
	repoFactory *outboxmocks.OutboxRepositoryFactory
	tx          *dbmocks.MockDBTX
}

func TestBudgetActivatedPublisherSuite(t *testing.T) {
	suite.Run(t, new(BudgetActivatedPublisherSuite))
}

func (s *BudgetActivatedPublisherSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.repoFactory = outboxmocks.NewOutboxRepositoryFactory(s.T())
	s.tx = dbmocks.NewMockDBTX(s.T())
}

func (s *BudgetActivatedPublisherSuite) TestPublish_SetsAggregateUserIDAndPayload() {
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	competence, err := valueobjects.NewCompetence("2026-06")
	s.Require().NoError(err)
	now := time.Date(2026, time.June, 17, 12, 30, 45, 0, time.UTC)
	budget := entities.NewBudget(userID, competence, 100000, now)
	budget.SetAllocations([]entities.Allocation{
		entities.NewAllocation(budget.ID(), valueobjects.RootSlugPrazeres, 10000, 100000),
	})
	s.Require().NoError(budget.Activate(now))

	s.repoFactory.EXPECT().
		OutboxRepository(s.tx).
		Return(s.storage).
		Once()

	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(e outbox.Event) bool {
			return e.Type == "budgets.budget_activated.v1" &&
				e.AggregateID == budget.ID().String() &&
				e.AggregateUserID == userID.String()
		}), 5).
		Return(nil).
		Once()

	cfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	publisher := producers.NewBudgetActivatedPublisher(s.repoFactory, cfg, id.NewUUIDGenerator(), noop.NewProvider())

	pubErr := publisher.Publish(context.Background(), s.tx, budget, now)
	s.NoError(pubErr)
}
