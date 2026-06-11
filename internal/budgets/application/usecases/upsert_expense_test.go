package usecases_test

import (
	"context"
	"errors"
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

type UpsertExpenseSuite struct {
	suite.Suite
	ctx        context.Context
	factory    *mockInterfaces.RepositoryFactory
	expenses   *mockInterfaces.ExpenseRepository
	budgets    *mockInterfaces.BudgetRepository
	categories *mockInterfaces.CategoriesReader
	publisher  *mockInterfaces.ExpenseCommittedPublisher
	autoDraft  *usecases.CreateOrAutoDraftForExpense
	uow        *uowMocks.UnitOfWorkExpense
	useCase    *usecases.UpsertExpense
}

func TestUpsertExpenseSuite(t *testing.T) {
	suite.Run(t, new(UpsertExpenseSuite))
}

func (s *UpsertExpenseSuite) SetupTest() {
	s.ctx = context.Background()
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.expenses = mockInterfaces.NewExpenseRepository(s.T())
	s.budgets = mockInterfaces.NewBudgetRepository(s.T())
	s.factory.EXPECT().ExpenseRepository(mock.Anything).Return(s.expenses).Maybe()
	s.factory.EXPECT().BudgetRepository(mock.Anything).Return(s.budgets).Maybe()
	s.categories = mockInterfaces.NewCategoriesReader(s.T())
	s.publisher = mockInterfaces.NewExpenseCommittedPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkExpense(s.T())
	s.autoDraft = usecases.NewCreateOrAutoDraftForExpense(s.factory)
	loc, _ := time.LoadLocation("America/Sao_Paulo")
	s.useCase = usecases.NewUpsertExpense(
		s.factory, s.categories, s.publisher, s.autoDraft, s.uow, noop.NewProvider(), loc,
	)
}

func (s *UpsertExpenseSuite) validInput() input.UpsertExpenseInput {
	return input.UpsertExpenseInput{
		UserID:                uuid.New().String(),
		Source:                "api",
		ExternalTransactionID: "00000000-0000-4000-8000-000000000001",
		SubcategoryID:         uuid.New().String(),
		Competence:            "2026-06",
		AmountCents:           5000,
		OccurredAt:            time.Now().UTC(),
	}
}

func (s *UpsertExpenseSuite) TestCreate_FreshExpense() {
	in := s.validInput()

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	s.budgets.EXPECT().
		GetByUserCompetence(s.ctx, mock.Anything, mock.Anything).
		Return(entities.Budget{}, interfaces.ErrBudgetNotFound).
		Once()

	s.budgets.EXPECT().
		CreateDraft(s.ctx, mock.Anything).
		Return(nil).
		Once()

	s.expenses.EXPECT().
		Insert(s.ctx, mock.Anything).
		Return(nil).
		Once()

	s.publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, in)

	s.NoError(err)
	s.Equal(in.UserID, result.UserID)
	s.Equal("expense.custo_fixo", result.RootSlug)
	s.Equal(int64(1), result.Version)
}

func (s *UpsertExpenseSuite) TestCreate_IdempotentRetry() {
	in := s.validInput()

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	source, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	competence, _ := valueobjects.NewCompetence(in.Competence)
	rootSlug, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	userID, _ := uuid.Parse(in.UserID)
	subID, _ := uuid.Parse(in.SubcategoryID)

	existing, _ := entities.NewExpense(userID, source, extID, subID, rootSlug, competence, 5000, time.Now().UTC(), time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, in)

	s.NoError(err)
	s.Equal(int64(1), result.Version)
}

func (s *UpsertExpenseSuite) TestCreate_WithExplicitVersionRejected() {
	in := s.validInput()
	v := int64(1)
	in.ExpectedVersion = &v

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, entities.ExpenseTombstone{}, interfaces.ErrExpenseNotFound).
		Once()

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseExplicitVersion)
}

func (s *UpsertExpenseSuite) TestCreate_TombstoneBlocksRecreation() {
	in := s.validInput()

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	source, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	userID, _ := uuid.Parse(in.UserID)
	tombstone := entities.NewExpenseTombstone(userID, source, extID, 2, time.Now().UTC())

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(entities.Expense{}, tombstone, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, interfaces.ErrExpenseTombstoneConflict)
}

func (s *UpsertExpenseSuite) TestUpdate_Success() {
	in := s.validInput()
	v := int64(1)
	in.ExpectedVersion = &v

	source, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	competence, _ := valueobjects.NewCompetence(in.Competence)
	rootSlug, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	userID, _ := uuid.Parse(in.UserID)
	subID, _ := uuid.Parse(in.SubcategoryID)

	existing, _ := entities.NewExpense(userID, source, extID, subID, rootSlug, competence, 5000, time.Now().UTC(), time.Now().UTC())

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.conhecimento", false, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	s.expenses.EXPECT().
		Update(s.ctx, mock.Anything, int64(1)).
		Return(nil).
		Once()

	s.publisher.EXPECT().
		Publish(s.ctx, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, in)

	s.NoError(err)
	s.Equal("expense.conhecimento", result.RootSlug)
}

func (s *UpsertExpenseSuite) TestUpdate_VersionConflict() {
	in := s.validInput()
	v := int64(99)
	in.ExpectedVersion = &v

	source, _ := valueobjects.NewProducerSource("api")
	extID, _ := valueobjects.NewExternalTransactionID(in.ExternalTransactionID)
	competence, _ := valueobjects.NewCompetence(in.Competence)
	rootSlug, _ := valueobjects.ParseRootSlug("expense.custo_fixo")
	userID, _ := uuid.Parse(in.UserID)
	subID, _ := uuid.Parse(in.SubcategoryID)

	existing, _ := entities.NewExpense(userID, source, extID, subID, rootSlug, competence, 5000, time.Now().UTC(), time.Now().UTC())

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("expense.custo_fixo", false, nil).
		Once()

	s.expenses.EXPECT().
		GetByIdentity(s.ctx, mock.Anything).
		Return(existing, entities.ExpenseTombstone{}, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, interfaces.ErrExpenseConflict)
}

func (s *UpsertExpenseSuite) TestCreate_InvalidUserID() {
	in := s.validInput()
	in.UserID = "invalid"

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidUserID)
}

func (s *UpsertExpenseSuite) TestCreate_InvalidSubcategoryID() {
	in := s.validInput()
	in.SubcategoryID = "not-a-uuid"

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidSubcategory)
}

func (s *UpsertExpenseSuite) TestCreate_InvalidSource() {
	in := s.validInput()
	in.Source = ""

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidSource)
}

func (s *UpsertExpenseSuite) TestCreate_InvalidCompetence() {
	in := s.validInput()
	in.Competence = "2026-13"

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidCompetence)
}

func (s *UpsertExpenseSuite) TestCreate_RejectsZeroAmount() {
	in := s.validInput()
	in.AmountCents = 0

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidAmount)
}

func (s *UpsertExpenseSuite) TestCreate_RejectsNegativeAmount() {
	in := s.validInput()
	in.AmountCents = -1

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidAmount)
}

func (s *UpsertExpenseSuite) TestUpdate_RejectsZeroAmount() {
	in := s.validInput()
	in.AmountCents = 0
	expectedVersion := int64(1)
	in.ExpectedVersion = &expectedVersion

	_, err := s.useCase.Execute(s.ctx, in)

	s.ErrorIs(err, usecases.ErrUpsertExpenseInvalidAmount)
}

func (s *UpsertExpenseSuite) TestCreate_CategoriesReaderError() {
	in := s.validInput()

	s.categories.EXPECT().
		ValidateExpenseSubcategory(s.ctx, mock.Anything).
		Return("", false, errors.New("categories unavailable")).
		Once()

	_, err := s.useCase.Execute(s.ctx, in)

	s.Error(err)
}
