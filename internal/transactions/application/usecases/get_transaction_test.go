package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetTransactionSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.TransactionRepository
	uow     *uowMocks.UnitOfWorkOutputTransaction
	useCase *GetTransaction
}

func TestGetTransactionSuite(t *testing.T) {
	suite.Run(t, new(GetTransactionSuite))
}

func (s *GetTransactionSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkOutputTransaction(s.T())
	s.useCase = NewGetTransaction(s.factory, s.uow, fake.NewProvider())
}

func (s *GetTransactionSuite) makeTransaction(txID uuid.UUID) *entities.Transaction {
	userID := valueobjects.UserIDFromUUID(s.userID)
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(5000)
	desc, _ := valueobjects.NewDescription("Supermercado")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	tx := entities.Reconstitute(
		txID, userID, dir, pm, amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "", valueobjects.CategoryWriteEvidence{}, rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	return &tx
}

func (s *GetTransactionSuite) TestExecute_Success() {
	txID := uuid.New()
	existing := s.makeTransaction(txID)

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()

	result, err := s.useCase.Execute(s.ctx, txID.String())
	s.Require().NoError(err)
	s.Equal(txID, result.ID)
	s.Equal(int64(5000), result.AmountCents)
}

func (s *GetTransactionSuite) TestExecute_NotFound_OtherUser() {
	txID := uuid.New()
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(nil, interfaces.ErrTransactionNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String())
	s.Require().Error(err)
}

func (s *GetTransactionSuite) TestExecute_Unauthorized() {
	_, err := s.useCase.Execute(context.Background(), uuid.NewString())
	s.Require().Error(err)
}

func (s *GetTransactionSuite) TestExecute_InvalidID() {
	_, err := s.useCase.Execute(s.ctx, "not-a-uuid")
	s.Require().Error(err)
}
