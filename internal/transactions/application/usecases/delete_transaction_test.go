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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type DeleteTransactionSuite struct {
	suite.Suite
	ctx         context.Context
	userID      uuid.UUID
	factory     *mockInterfaces.RepositoryFactory
	repo        *mockInterfaces.TransactionRepository
	invoiceRepo *mockInterfaces.CardInvoiceRepository
	publisher   *mockInterfaces.TransactionEventPublisher
	uow         *uowMocks.UnitOfWorkVoid
	useCase     *DeleteTransaction
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
	s.invoiceRepo = mockInterfaces.NewCardInvoiceRepository(s.T())
	s.factory.EXPECT().CardInvoiceRepository(mock.Anything).Return(s.invoiceRepo).Maybe()
	s.publisher = mockInterfaces.NewTransactionEventPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = NewDeleteTransaction(s.factory, s.uow, services.TransactionWorkflow{}, s.publisher, fake.NewProvider())
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
		"Cat", "", rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	return &tx
}

func (s *DeleteTransactionSuite) TestExecute_Success() {
	txID := uuid.New()
	existing := s.makeExistingTransaction(txID)

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
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
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
	s.repo.EXPECT().SoftDelete(mock.Anything, txID, s.userID, int64(1), mock.Anything).Return(interfaces.ErrTransactionVersionConflict).Once()

	err := s.useCase.Execute(s.ctx, txID.String(), 1)
	s.Require().Error(err)
}

func (s *DeleteTransactionSuite) TestExecute_CreditCard_RevertsDeltas() {
	txID := uuid.New()
	cardID := uuid.New()
	userIDVO := valueobjects.UserIDFromUUID(s.userID)
	cardIDVO := valueobjects.CardIDFromUUID(cardID)
	amount, _ := valueobjects.NewMoney(30000)
	desc, _ := valueobjects.NewDescription("Notebook")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	rm, _ := valueobjects.NewRefMonth("2026-07")
	snapshot, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	installments, _ := valueobjects.NewInstallmentCount(2)
	now := time.Now().UTC()

	existing := entities.Reconstitute(
		txID, userIDVO, valueobjects.DirectionOutcome, valueobjects.PaymentMethodCreditCard,
		amount, desc, catID, option.None[valueobjects.SubcategoryID](),
		"Compras", "Eletrônicos", rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	existing.SetCardBilling(cardIDVO, installments, snapshot)

	itemAmount, _ := valueobjects.NewMoney(15000)
	item := entities.NewCardInvoiceItem(uuid.New(), uuid.New(), txID, userIDVO, rm, 1, itemAmount, now)
	inv := entities.NewCardInvoice(uuid.New(), userIDVO, cardIDVO, rm, now, now, now)

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(&existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return([]*entities.CardInvoiceItem{&item}, nil).Once()
	s.repo.EXPECT().SoftDelete(mock.Anything, txID, s.userID, int64(1), mock.Anything).Return(nil).Once()
	s.invoiceRepo.EXPECT().GetByMonth(mock.Anything, s.userID, cardID, rm).Return(&inv, nil, nil).Once()
	s.invoiceRepo.EXPECT().ApplyDelta(mock.Anything, inv.ID(), int64(-15000), int64(1)).Return(nil).Once()
	s.repo.EXPECT().ReplaceItems(mock.Anything, txID, ([]*entities.CardInvoiceItem)(nil)).Return(nil).Once()
	s.publisher.EXPECT().PublishDeleted(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	err := s.useCase.Execute(s.ctx, txID.String(), 1)
	s.Require().NoError(err)
}
