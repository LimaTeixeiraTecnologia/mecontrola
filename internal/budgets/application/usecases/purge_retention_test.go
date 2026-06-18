package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
)

type PurgeRetentionSuite struct {
	suite.Suite
	ctx      context.Context
	factory  *mockInterfaces.RepositoryFactory
	pending  *mockInterfaces.PendingEventRepository
	expenses *mockInterfaces.ExpenseRepository
	alerts   *mockInterfaces.AlertRepository
	uow      *uowMocks.UnitOfWorkVoid
	useCase  *usecases.PurgeRetention
}

func TestPurgeRetentionSuite(t *testing.T) {
	suite.Run(t, new(PurgeRetentionSuite))
}

func (s *PurgeRetentionSuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.pending = mockInterfaces.NewPendingEventRepository(s.T())
	s.expenses = mockInterfaces.NewExpenseRepository(s.T())
	s.alerts = mockInterfaces.NewAlertRepository(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())

	s.factory.EXPECT().PendingEventRepository(mock.Anything).Return(s.pending).Maybe()
	s.factory.EXPECT().ExpenseRepository(mock.Anything).Return(s.expenses).Maybe()
	s.factory.EXPECT().AlertRepository(mock.Anything).Return(s.alerts).Maybe()

	s.useCase = usecases.NewPurgeRetention(s.factory, s.uow, 250, noop.NewProvider())
}

func (s *PurgeRetentionSuite) TestExecute_DefersWhenPendingExists() {
	s.pending.EXPECT().
		ListReady(s.ctx, 1).
		Return([]entities.PendingEvent{entities.PendingEvent{}}, nil).
		Once()

	err := s.useCase.Execute(s.ctx)

	s.NoError(err)
}

func (s *PurgeRetentionSuite) TestExecute_PurgesExpensesAndAlerts() {
	s.pending.EXPECT().
		ListReady(s.ctx, 1).
		Return(nil, nil).
		Once()
	s.expenses.EXPECT().
		PurgeDeleted(s.ctx, "24 months", 250).
		Return(int64(2), nil).
		Once()
	s.alerts.EXPECT().
		PurgeOld(s.ctx, "24 months", 250).
		Return(int64(3), nil).
		Once()

	err := s.useCase.Execute(s.ctx)

	s.NoError(err)
}

func (s *PurgeRetentionSuite) TestExecute_ExpensePurgeError() {
	s.pending.EXPECT().
		ListReady(s.ctx, 1).
		Return(nil, nil).
		Once()
	s.expenses.EXPECT().
		PurgeDeleted(s.ctx, "24 months", 250).
		Return(int64(0), errors.New("db boom")).
		Once()

	err := s.useCase.Execute(s.ctx)

	s.Error(err)
}
