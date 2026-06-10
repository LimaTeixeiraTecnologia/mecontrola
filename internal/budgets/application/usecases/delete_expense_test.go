package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type DeleteExpenseSuite struct {
	suite.Suite
	ctx       context.Context
	expenses  *mockInterfaces.ExpenseRepository
	publisher *mockInterfaces.ExpenseCommittedPublisher
	uow       *uowMocks.UnitOfWorkVoid
	useCase   *usecases.DeleteExpense
}

func TestDeleteExpenseSuite(t *testing.T) {
	suite.Run(t, new(DeleteExpenseSuite))
}

func (s *DeleteExpenseSuite) SetupTest() {
	s.ctx = context.Background()
	s.expenses = mockInterfaces.NewExpenseRepository(s.T())
	s.publisher = mockInterfaces.NewExpenseCommittedPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	s.useCase = usecases.NewDeleteExpense(s.expenses, s.publisher, s.uow, noop.NewProvider(), loc)
}

func (s *DeleteExpenseSuite) buildExisting(userID uuid.UUID, extIDStr string) entities.Expense {
	source, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(extIDStr)
	competence, _ := valueobjects.NewCompetence("2026-06")
	rootSlug, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	subID := uuid.New()
	e, _ := entities.NewExpense(userID, source, extID, subID, rootSlug, competence, 5000, time.Now().UTC(), time.Now().UTC())
	return e
}

func (s *DeleteExpenseSuite) validInput(userID uuid.UUID) input.DeleteExpenseInput {
	return input.DeleteExpenseInput{
		UserID:                userID.String(),
		Source:                "api",
		ExternalTransactionID: "00000000-0000-4000-8000-000000000001",
		ExpectedVersion:       1,
	}
}

func (s *DeleteExpenseSuite) TestSoftDelete_Success() {
	userID := uuid.New()
	in := s.validInput(userID)
	existing := s.buildExisting(userID, in.ExternalTransactionID)

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.expenses.EXPECT().
		SoftDelete(s.ctx, mock.Anything, mock.Anything, int64(1)).
		Return(int64(2), nil).
		Once()

	s.publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	err := s.useCase.Execute(s.ctx, in)

	s.NoError(err)
}

func (s *DeleteExpenseSuite) TestSoftDelete_IdempotentRetry() {
	userID := uuid.New()
	in := s.validInput(userID)

	source, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	tombstone := entities.NewExpenseTombstone(userID, source, extID, 2, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything, mock.Anything).
		Return(entities.Expense{}, tombstone, nil).
		Once()

	err := s.useCase.Execute(s.ctx, in)

	s.NoError(err)
}

func (s *DeleteExpenseSuite) TestSoftDelete_VersionConflict() {
	userID := uuid.New()
	in := s.validInput(userID)
	in.ExpectedVersion = 99
	existing := s.buildExisting(userID, in.ExternalTransactionID)

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.expenses.EXPECT().
		SoftDelete(s.ctx, mock.Anything, mock.Anything, int64(99)).
		Return(int64(0), interfaces.ErrExpenseConflict).
		Once()

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, interfaces.ErrExpenseConflict)
}

func (s *DeleteExpenseSuite) TestSoftDelete_NotFound() {
	userID := uuid.New()
	in := s.validInput(userID)

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, interfaces.ErrExpenseNotFound)
}

func (s *DeleteExpenseSuite) TestSoftDelete_InvalidUserID() {
	in := input.DeleteExpenseInput{
		UserID:                "not-a-uuid",
		Source:                "api",
		ExternalTransactionID: "00000000-0000-4000-8000-000000000001",
		ExpectedVersion:       1,
	}

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrDeleteExpenseInvalidUserID)
}

func (s *DeleteExpenseSuite) TestSoftDelete_InvalidSource() {
	in := input.DeleteExpenseInput{
		UserID:                uuid.New().String(),
		Source:                "",
		ExternalTransactionID: "00000000-0000-4000-8000-000000000001",
		ExpectedVersion:       1,
	}

	err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrDeleteExpenseInvalidSource)
}
