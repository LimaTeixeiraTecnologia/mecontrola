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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/events"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/infrastructure/messaging/database/producers"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
	outboxmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox/mocks"
)

type ExpenseCommittedPublisherSuite struct {
	suite.Suite
	storage     *outboxmocks.Storage
	repoFactory *outboxmocks.OutboxRepositoryFactory
	tx          *dbmocks.MockDBTX
}

func TestExpenseCommittedPublisherSuite(t *testing.T) {
	suite.Run(t, new(ExpenseCommittedPublisherSuite))
}

func (s *ExpenseCommittedPublisherSuite) SetupTest() {
	s.storage = outboxmocks.NewStorage(s.T())
	s.repoFactory = outboxmocks.NewOutboxRepositoryFactory(s.T())
	s.tx = dbmocks.NewMockDBTX(s.T())
}

func (s *ExpenseCommittedPublisherSuite) TestPublish_SetsAggregateUserID() {
	expenseID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	userID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	subcategoryID := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	competence, err := valueobjects.NewCompetence("2025-06")
	s.Require().NoError(err)
	cutoff, err := valueobjects.NewCompetence("2025-05")
	s.Require().NoError(err)
	committedAt := time.Date(2025, 6, 15, 12, 30, 45, 0, time.UTC)

	evt, err := events.NewExpenseCommitted(
		expenseID, userID, subcategoryID,
		valueobjects.RootSlugCustoFixo, competence,
		valueobjects.MutationKindCreate, committedAt, cutoff,
	)
	s.Require().NoError(err)

	expectedUserID := userID.String()

	s.repoFactory.EXPECT().
		OutboxRepository(s.tx).
		Return(s.storage).
		Once()

	s.storage.EXPECT().
		Insert(mock.Anything, mock.MatchedBy(func(e outbox.Event) bool {
			return e.AggregateUserID == expectedUserID
		}), 5).
		Return(nil).
		Once()

	cfg := configs.OutboxConfig{RetryMaxAttempts: 5}
	idGen := id.NewUUIDGenerator()
	publisher := producers.NewExpenseCommittedPublisher(s.repoFactory, cfg, idGen, noop.NewProvider())

	pubErr := publisher.Publish(context.Background(), s.tx, evt)
	s.NoError(pubErr)
}
