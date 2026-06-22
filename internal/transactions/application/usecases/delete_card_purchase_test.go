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
	ifmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type DeleteCardPurchaseSuite struct {
	suite.Suite
	uc        *DeleteCardPurchase
	factory   *ifmocks.RepositoryFactory
	purchases *ifmocks.CardPurchaseRepository
	invoices  *ifmocks.CardInvoiceRepository
	publisher *ifmocks.CardPurchaseEventPublisher
	snapshot  valueobjects.CardBillingSnapshot
}

func TestDeleteCardPurchaseSuite(t *testing.T) {
	suite.Run(t, new(DeleteCardPurchaseSuite))
}

func (s *DeleteCardPurchaseSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.purchases = ifmocks.NewCardPurchaseRepository(s.T())
	s.invoices = ifmocks.NewCardInvoiceRepository(s.T())
	s.publisher = ifmocks.NewCardPurchaseEventPublisher(s.T())

	s.factory.On("CardPurchaseRepository", mock.Anything).Return(s.purchases).Maybe()
	s.factory.On("CardInvoiceRepository", mock.Anything).Return(s.invoices).Maybe()

	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	s.snapshot = snap

	wf := services.NewCardPurchaseWorkflow()
	uow := ucmocks.NewUnitOfWorkCardPurchase(s.T())
	idGen := &testIDGen{}
	s.uc = NewDeleteCardPurchase(
		s.factory, &wf, s.publisher, uow, idGen, fake.NewProvider(),
	)
}

func (s *DeleteCardPurchaseSuite) TestExecute_Unauthorized() {
	err := s.uc.Execute(context.Background(), uuid.New(), 1)
	s.ErrorIs(err, ErrUsecaseUnauthorized)
}

func (s *DeleteCardPurchaseSuite) TestExecute_SoftDeletesAndPublishes() {
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(), Source: auth.SourceHeader,
	})
	principal, _ := auth.FromContext(ctx)

	purchaseID := uuid.New()
	cardID := uuid.New()
	catID := uuid.New()
	catVo, _ := valueobjects.ParseCategoryID(catID.String())
	amount, _ := valueobjects.NewMoney(1000)
	inst, _ := valueobjects.NewInstallmentCount(1)
	desc, _ := valueobjects.NewDescription("Compra")
	purchasedAt := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)

	p := entities.NewCardPurchase(
		purchaseID,
		valueobjects.UserIDFromUUID(principal.UserID),
		valueobjects.CardIDFromUUID(cardID),
		amount, inst, desc, catVo,
		option.None[valueobjects.SubcategoryID](),
		"", "", purchasedAt, s.snapshot, time.Now(),
	)

	rm := valueobjects.RefMonthFromTime(purchasedAt, time.UTC)
	invItem := entities.NewCardInvoiceItem(
		uuid.New(), uuid.New(), purchaseID,
		valueobjects.UserIDFromUUID(principal.UserID),
		rm, 1, amount, time.Now(),
	)

	inv := entities.NewCardInvoice(
		uuid.New(), valueobjects.UserIDFromUUID(principal.UserID),
		valueobjects.CardIDFromUUID(cardID), rm,
		purchasedAt.AddDate(0, 0, 10), purchasedAt.AddDate(0, 0, 20), time.Now(),
	)

	s.purchases.On("GetByID", mock.Anything, purchaseID, principal.UserID).Return(&p, nil)
	s.invoices.On("GetByMonth", mock.Anything, principal.UserID, cardID, rm).
		Return(&inv, []*entities.CardInvoiceItem{&invItem}, nil).Maybe()
	s.purchases.On("SoftDelete", mock.Anything, purchaseID, principal.UserID, int64(1), mock.Anything).Return(nil)
	s.purchases.On("ReplaceItems", mock.Anything, purchaseID, ([]*entities.CardInvoiceItem)(nil)).Return(nil)
	s.invoices.On("ApplyDelta", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()
	s.publisher.On("PublishDeleted", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := s.uc.Execute(ctx, purchaseID, 1)
	s.NoError(err)
}
