package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type EvaluateThresholdAlertsSuite struct {
	suite.Suite
	ctx       context.Context
	obs       observability.Observability
	factory   *mockInterfaces.RepositoryFactory
	sentRepo  *mockInterfaces.ThresholdAlertSentRepository
	publisher *mockInterfaces.ThresholdAlertPublisher
	uow       *uowMocks.UnitOfWorkVoid
	useCase   *EvaluateThresholdAlerts
}

func TestEvaluateThresholdAlertsSuite(t *testing.T) {
	suite.Run(t, new(EvaluateThresholdAlertsSuite))
}

func (s *EvaluateThresholdAlertsSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.sentRepo = mockInterfaces.NewThresholdAlertSentRepository(s.T())
	s.publisher = mockInterfaces.NewThresholdAlertPublisher(s.T())
	s.factory.EXPECT().ThresholdAlertSentRepository(mock.Anything).Return(s.sentRepo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())

	cfg := services.ThresholdConfig{
		Category: valueobjects.MustThresholdRatio(0.80),
		Goal:     valueobjects.MustThresholdRatio(0.50),
	}
	s.useCase = NewEvaluateThresholdAlerts(
		s.factory,
		s.publisher,
		s.uow,
		cfg,
		time.UTC,
		100,
		s.obs,
	)
}

func (s *EvaluateThresholdAlertsSuite) TestExecute_NoActiveBudgets_NoOp() {
	s.sentRepo.EXPECT().
		ListActiveForThresholdScan(mock.Anything, mock.Anything, 100).
		Return(nil, nil).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func (s *EvaluateThresholdAlertsSuite) TestExecute_DispatchesCategoryAlert() {
	userID := uuid.New()
	budgetID := uuid.New()
	comp := valueobjects.CompetenceFromTime(time.Now().UTC(), time.UTC)

	active := []interfaces.ActiveBudgetForScan{
		{
			UserID:       userID,
			BudgetID:     budgetID,
			Competence:   comp,
			RootSlug:     valueobjects.RootSlugCustoFixo,
			PlannedCents: 1000,
			SpentCents:   900,
		},
	}

	s.sentRepo.EXPECT().
		ListActiveForThresholdScan(mock.Anything, mock.Anything, 100).
		Return(active, nil).
		Once()

	s.sentRepo.EXPECT().
		ListSentForDay(mock.Anything, mock.Anything).
		Return(nil, nil).
		Once()

	s.publisher.EXPECT().
		Publish(mock.Anything, mock.Anything, mock.MatchedBy(func(alert services.DomainAlert) bool {
			return alert.UserID == userID &&
				alert.BudgetID == budgetID &&
				alert.Kind == services.ThresholdAlertCategory &&
				alert.PercentUsedBps == 9000 &&
				alert.AmountRemainingCents == 100
		}), mock.Anything).
		Return(nil).
		Once()

	s.sentRepo.EXPECT().
		InsertSent(mock.Anything, mock.MatchedBy(func(rec interfaces.ThresholdAlertSentRecord) bool {
			return rec.UserID == userID &&
				rec.BudgetID == budgetID &&
				rec.Kind == services.ThresholdAlertCategory
		})).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func (s *EvaluateThresholdAlertsSuite) TestExecute_GoalKindWhenRootIsMetas() {
	userID := uuid.New()
	budgetID := uuid.New()
	comp := valueobjects.CompetenceFromTime(time.Now().UTC(), time.UTC)

	active := []interfaces.ActiveBudgetForScan{
		{
			UserID:       userID,
			BudgetID:     budgetID,
			Competence:   comp,
			RootSlug:     valueobjects.RootSlugMetas,
			PlannedCents: 1000,
			SpentCents:   500,
		},
	}

	s.sentRepo.EXPECT().ListActiveForThresholdScan(mock.Anything, mock.Anything, 100).Return(active, nil).Once()
	s.sentRepo.EXPECT().ListSentForDay(mock.Anything, mock.Anything).Return(nil, nil).Once()
	s.publisher.EXPECT().
		Publish(mock.Anything, mock.Anything, mock.MatchedBy(func(a services.DomainAlert) bool {
			return a.Kind == services.ThresholdAlertGoal
		}), mock.Anything).
		Return(nil).
		Once()
	s.sentRepo.EXPECT().InsertSent(mock.Anything, mock.Anything).Return(nil).Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func (s *EvaluateThresholdAlertsSuite) TestExecute_DedupsAlreadySent() {
	userID := uuid.New()
	budgetID := uuid.New()
	comp := valueobjects.CompetenceFromTime(time.Now().UTC(), time.UTC)
	day := time.Now().UTC().Truncate(24 * time.Hour)

	active := []interfaces.ActiveBudgetForScan{
		{
			UserID:       userID,
			BudgetID:     budgetID,
			Competence:   comp,
			RootSlug:     valueobjects.RootSlugCustoFixo,
			PlannedCents: 1000,
			SpentCents:   900,
		},
	}

	s.sentRepo.EXPECT().ListActiveForThresholdScan(mock.Anything, mock.Anything, 100).Return(active, nil).Once()
	s.sentRepo.EXPECT().
		ListSentForDay(mock.Anything, mock.Anything).
		Return([]interfaces.ThresholdAlertSentRecord{
			{
				UserID:   userID,
				BudgetID: budgetID,
				Kind:     services.ThresholdAlertCategory,
				RefDay:   day,
				SentAt:   time.Now().UTC(),
			},
		}, nil).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.NoError(err)
}

func (s *EvaluateThresholdAlertsSuite) TestExecute_ListActiveError_Propagates() {
	s.sentRepo.EXPECT().
		ListActiveForThresholdScan(mock.Anything, mock.Anything, 100).
		Return(nil, errors.New("db boom")).
		Once()

	err := s.useCase.Execute(s.ctx)
	s.Error(err)
}

func (s *EvaluateThresholdAlertsSuite) TestExecute_PublishError_Propagates() {
	userID := uuid.New()
	budgetID := uuid.New()
	comp := valueobjects.CompetenceFromTime(time.Now().UTC(), time.UTC)

	active := []interfaces.ActiveBudgetForScan{
		{
			UserID:       userID,
			BudgetID:     budgetID,
			Competence:   comp,
			RootSlug:     valueobjects.RootSlugCustoFixo,
			PlannedCents: 1000,
			SpentCents:   900,
		},
	}

	s.sentRepo.EXPECT().ListActiveForThresholdScan(mock.Anything, mock.Anything, 100).Return(active, nil).Once()
	s.sentRepo.EXPECT().ListSentForDay(mock.Anything, mock.Anything).Return(nil, nil).Once()
	s.publisher.EXPECT().Publish(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(errors.New("outbox down")).Once()

	err := s.useCase.Execute(s.ctx)
	s.Error(err)
}
