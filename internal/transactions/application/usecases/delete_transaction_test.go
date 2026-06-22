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

type DeleteTransactionSuite struct {
	suite.Suite
	ctx       context.Context
	userID    uuid.UUID
	factory   *mockInterfaces.RepositoryFactory
	repo      *mockInterfaces.TransactionRepository
	publisher *mockInterfaces.TransactionEventPublisher
	uow       *uowMocks.UnitOfWorkVoid
	useCase   *DeleteTransaction
}

func TestDeleteTransactionSuite(t *testing.T) {
	suite.Run(t, new(DeleteTransactionSuite))
}

func (s *DeleteTransactionSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.publisher = mockInterfaces.NewTransactionEventPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = NewDeleteTransaction(s.factory, s.uow, s.publisher, fake.NewProvider())
}

func (s *DeleteTransactionSuite) makeExistingTransaction(txID uuid.UUID) *entities.Transaction {
	userID := valueobjects.UserIDFromUUID(s.userID)
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(1000)
	desc, _ := valueobjects.NewDescription("Mercado")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-06")
	now := time.Now().UTC()

	tx := entities.Reconstitute(
		txID, userID, dir, pm, amount, desc, catID,
		option.None[valueobjects.SubcategoryID](),
		"Cat", "", rm, now, 1, nil, now, now,
	)
	return &tx
}

func (s *DeleteTransactionSuite) TestExecute_Success() {
	txID := uuid.New()
	existing := s.makeExistingTransaction(txID)

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().SoftDelete(mock.Anything, txID, s.userID, int64(1), mock.Anything).Return(nil).Once()
	s.publisher.EXPECT().PublishDeleted(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	err := s.useCase.Execute(s.ctx, txID.String(), 1)
	s.Require().NoError(err)
}

func (s *DeleteTransactionSuite) TestExecute_Unauthorized() {
	err := s.useCase.Execute(context.Background(), uuid.NewString(), 1)
	s.Require().Error(err)
}

func (s *DeleteTransactionSuite) TestExecute_InvalidID() {
	err := s.useCase.Execute(s.ctx, "not-a-uuid", 1)
	s.Require().Error(err)
}

func (s *DeleteTransactionSuite) TestExecute_NotFound() {
	txID := uuid.New()
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(nil, interfaces.ErrTransactionNotFound).Once()

	err := s.useCase.Execute(s.ctx, txID.String(), 1)
	s.Require().Error(err)
}

func (s *DeleteTransactionSuite) TestExecute_SoftDeleteError_VersionConflict() {
	txID := uuid.New()
	existing := s.makeExistingTransaction(txID)

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().SoftDelete(mock.Anything, txID, s.userID, int64(1), mock.Anything).Return(interfaces.ErrTransactionVersionConflict).Once()

	err := s.useCase.Execute(s.ctx, txID.String(), 1)
	s.Require().Error(err)
}
