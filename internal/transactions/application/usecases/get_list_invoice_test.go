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
	ifmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetCardPurchaseSuite struct {
	suite.Suite
	uc        *usecases.GetCardPurchase
	factory   *ifmocks.RepositoryFactory
	purchases *ifmocks.CardPurchaseRepository
}

func TestGetCardPurchaseSuite(t *testing.T) {
	suite.Run(t, new(GetCardPurchaseSuite))
}

func (s *GetCardPurchaseSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.purchases = ifmocks.NewCardPurchaseRepository(s.T())
	s.factory.On("CardPurchaseRepository", mock.Anything).Return(s.purchases).Maybe()
	uow := ucmocks.NewUnitOfWorkCardPurchaseOutput(s.T())
	s.uc = usecases.NewGetCardPurchase(s.factory, uow, noop.NewProvider())
}

func (s *GetCardPurchaseSuite) TestExecute_Unauthorized() {
	_, err := s.uc.Execute(context.Background(), uuid.New())
	s.ErrorIs(err, usecases.ErrUsecaseUnauthorized)
}

func (s *GetCardPurchaseSuite) TestExecute_Success() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	principal, _ := auth.FromContext(ctx)
	purchaseID := uuid.New()
	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	amt, _ := valueobjects.NewMoney(1000)
	inst, _ := valueobjects.NewInstallmentCount(1)
	desc, _ := valueobjects.NewDescription("test")
	catVo := valueobjects.CategoryIDFromUUID(uuid.New())
	p := entities.NewCardPurchase(
		purchaseID, valueobjects.UserIDFromUUID(principal.UserID),
		valueobjects.CardIDFromUUID(uuid.New()),
		amt, inst, desc, catVo, option.None[valueobjects.SubcategoryID](),
		"", "", time.Now(), snap, time.Now(),
	)
	s.purchases.On("GetByID", mock.Anything, purchaseID, principal.UserID).Return(&p, nil)
	out, err := s.uc.Execute(ctx, purchaseID)
	s.NoError(err)
	s.Equal(purchaseID, out.ID)
}

type ListCardPurchasesSuite struct {
	suite.Suite
	uc        *usecases.ListCardPurchases
	factory   *ifmocks.RepositoryFactory
	purchases *ifmocks.CardPurchaseRepository
}

func TestListCardPurchasesSuite(t *testing.T) {
	suite.Run(t, new(ListCardPurchasesSuite))
}

func (s *ListCardPurchasesSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.purchases = ifmocks.NewCardPurchaseRepository(s.T())
	s.factory.On("CardPurchaseRepository", mock.Anything).Return(s.purchases).Maybe()
	uow := ucmocks.NewUnitOfWorkListCardPurchases(s.T())
	s.uc = usecases.NewListCardPurchases(s.factory, uow, noop.NewProvider())
}

func (s *ListCardPurchasesSuite) TestExecute_Unauthorized() {
	_, err := s.uc.Execute(context.Background(), usecases.ListCardPurchasesInput{})
	s.ErrorIs(err, usecases.ErrUsecaseUnauthorized)
}

func (s *ListCardPurchasesSuite) TestExecute_EmptyList() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	principal, _ := auth.FromContext(ctx)
	s.purchases.On("ListByCardAndMonth", mock.Anything, principal.UserID, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*entities.CardPurchase{}, interfaces.Cursor{}, nil)
	out, err := s.uc.Execute(ctx, usecases.ListCardPurchasesInput{CardID: uuid.New()})
	s.NoError(err)
	s.Empty(out.Items)
}

type GetCardInvoiceSuite struct {
	suite.Suite
	uc       *usecases.GetCardInvoice
	factory  *ifmocks.RepositoryFactory
	invoices *ifmocks.CardInvoiceRepository
}

func TestGetCardInvoiceSuite(t *testing.T) {
	suite.Run(t, new(GetCardInvoiceSuite))
}

func (s *GetCardInvoiceSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.invoices = ifmocks.NewCardInvoiceRepository(s.T())
	s.factory.On("CardInvoiceRepository", mock.Anything).Return(s.invoices).Maybe()
	uow := ucmocks.NewUnitOfWorkCardInvoiceOutput(s.T())
	s.uc = usecases.NewGetCardInvoice(s.factory, uow, noop.NewProvider())
}

func (s *GetCardInvoiceSuite) TestExecute_Unauthorized() {
	_, err := s.uc.Execute(context.Background(), uuid.New(), "2024-01")
	s.ErrorIs(err, usecases.ErrUsecaseUnauthorized)
}

func (s *GetCardInvoiceSuite) TestExecute_InvalidRefMonth() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	_, err := s.uc.Execute(ctx, uuid.New(), "invalid")
	s.Error(err)
}

func (s *GetCardInvoiceSuite) TestExecute_NotFound_ReturnsError() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	principal, _ := auth.FromContext(ctx)
	rm, _ := valueobjects.NewRefMonth("2024-01")
	s.invoices.On("GetByMonth", mock.Anything, principal.UserID, mock.Anything, rm).
		Return((*entities.CardInvoice)(nil), ([]*entities.CardInvoiceItem)(nil), nil)
	_, err := s.uc.Execute(ctx, uuid.New(), "2024-01")
	s.ErrorIs(err, usecases.ErrCardInvoiceNotFound)
}
