package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type SearchTransactionsSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.TransactionRepository
	uow     *uowMocks.UnitOfWorkOutputTransaction
	useCase *SearchTransactions
}

func TestSearchTransactionsSuite(t *testing.T) {
	suite.Run(t, new(SearchTransactionsSuite))
}

func (s *SearchTransactionsSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkOutputTransaction(s.T())
	s.useCase = NewSearchTransactions(s.factory, s.uow, fake.NewProvider())
}

func (s *SearchTransactionsSuite) makeTransaction(desc string) *entities.Transaction {
	userID := valueobjects.UserIDFromUUID(s.userID)
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(3500)
	d, _ := valueobjects.NewDescription(desc)
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()
	tx := entities.Reconstitute(
		uuid.New(), userID, dir, pm, amount, d, catID,
		option.None[valueobjects.SubcategoryID](),
		"Cat", "", rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	return &tx
}

func (s *SearchTransactionsSuite) TestExecute_Success() {
	txs := []*entities.Transaction{s.makeTransaction("Uber")}
	q, _ := valueobjects.NewSearchQuery("uber")
	s.repo.EXPECT().
		SearchByDescription(mock.Anything, s.userID, q, option.None[valueobjects.RefMonth](), 10).
		Return(txs, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, "uber", "", 0)
	s.Require().NoError(err)
	s.Len(result, 1)
	s.Equal("Uber", result[0].Description)
}

func (s *SearchTransactionsSuite) TestExecute_WithRefMonth() {
	txs := []*entities.Transaction{s.makeTransaction("Mercado")}
	q, _ := valueobjects.NewSearchQuery("mercado")
	rm, _ := valueobjects.NewRefMonth("2026-06")
	s.repo.EXPECT().
		SearchByDescription(mock.Anything, s.userID, q, option.Some(rm), 10).
		Return(txs, nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, "mercado", "2026-06", 0)
	s.Require().NoError(err)
	s.Len(result, 1)
}

func (s *SearchTransactionsSuite) TestExecute_LimitClamped() {
	q, _ := valueobjects.NewSearchQuery("uber")
	s.repo.EXPECT().
		SearchByDescription(mock.Anything, s.userID, q, option.None[valueobjects.RefMonth](), 10).
		Return(nil, nil).
		Once()

	_, err := s.useCase.Execute(s.ctx, "uber", "", 999)
	s.Require().NoError(err)
}

func (s *SearchTransactionsSuite) TestExecute_EmptyQuery() {
	_, err := s.useCase.Execute(s.ctx, "  ", "", 0)
	s.Require().Error(err)
}

func (s *SearchTransactionsSuite) TestExecute_ShortQuery() {
	_, err := s.useCase.Execute(s.ctx, "a", "", 0)
	s.Require().Error(err)
}

func (s *SearchTransactionsSuite) TestExecute_Unauthorized() {
	_, err := s.useCase.Execute(context.Background(), "uber", "", 0)
	s.Require().Error(err)
}

func (s *SearchTransactionsSuite) TestExecute_InvalidRefMonth() {
	_, err := s.useCase.Execute(s.ctx, "uber", "bad", 0)
	s.Require().Error(err)
}

func (s *SearchTransactionsSuite) TestExecute_RepoError() {
	q, _ := valueobjects.NewSearchQuery("uber")
	s.repo.EXPECT().
		SearchByDescription(mock.Anything, s.userID, q, option.None[valueobjects.RefMonth](), 10).
		Return(nil, errors.New("db down")).
		Once()

	_, err := s.useCase.Execute(s.ctx, "uber", "", 0)
	s.Require().Error(err)
}
