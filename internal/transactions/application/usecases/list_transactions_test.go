package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ListTransactionsSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.TransactionRepository
	uow     *uowMocks.UnitOfWorkTransactionPage
	useCase *usecases.ListTransactions
}

func TestListTransactionsSuite(t *testing.T) {
	suite.Run(t, new(ListTransactionsSuite))
}

func (s *ListTransactionsSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkTransactionPage(s.T())
	s.useCase = usecases.NewListTransactions(s.factory, s.uow, noop.NewProvider())
}

func (s *ListTransactionsSuite) makeTransactions(count int) []*entities.Transaction {
	userID := valueobjects.UserIDFromUUID(s.userID)
	result := make([]*entities.Transaction, count)
	for i := range result {
		dir := valueobjects.DirectionOutcome
		pm := valueobjects.PaymentMethodPix
		amount, _ := valueobjects.NewMoney(1000)
		desc, _ := valueobjects.NewDescription("Item")
		catID := valueobjects.CategoryIDFromUUID(uuid.New())
		rm, _ := valueobjects.NewRefMonth("2026-06")
		now := time.Now().UTC()
		tx := entities.Reconstitute(
			uuid.New(), userID, dir, pm, amount, desc, catID,
			option.None[valueobjects.SubcategoryID](),
			"Cat", "", rm, now, 1, nil, now, now,
		)
		result[i] = &tx
	}
	return result
}

func (s *ListTransactionsSuite) TestExecute_Success() {
	txs := s.makeTransactions(3)
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.repo.EXPECT().
		ListByMonth(mock.Anything, s.userID, rm, interfaces.Cursor{}, 50).
		Return(txs, interfaces.Cursor{}, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, "2026-06", "", 0)
	s.Require().NoError(err)
	s.Len(result.Transactions, 3)
	s.Empty(result.NextCursor)
}

func (s *ListTransactionsSuite) TestExecute_LimitClamped() {
	txs := s.makeTransactions(2)
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.repo.EXPECT().
		ListByMonth(mock.Anything, s.userID, rm, interfaces.Cursor{}, 200).
		Return(txs, interfaces.Cursor{}, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, "2026-06", "", 999)
	s.Require().NoError(err)
	s.Len(result.Transactions, 2)
}

func (s *ListTransactionsSuite) TestExecute_Unauthorized() {
	_, err := s.useCase.Execute(context.Background(), "2026-06", "", 0)
	s.Require().Error(err)
}

func (s *ListTransactionsSuite) TestExecute_InvalidRefMonth() {
	_, err := s.useCase.Execute(s.ctx, "invalid", "", 0)
	s.Require().Error(err)
}
