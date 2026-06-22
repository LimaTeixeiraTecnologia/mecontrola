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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	ifmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	ucmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CreateCardPurchaseSuite struct {
	suite.Suite
	uc           *CreateCardPurchase
	factory      *ifmocks.RepositoryFactory
	purchases    *ifmocks.CardPurchaseRepository
	invoices     *ifmocks.CardInvoiceRepository
	cardLookup   *ifmocks.CardLookup
	catValidator *ifmocks.CategoryValidator
	publisher    *ifmocks.CardPurchaseEventPublisher
	snapshot     valueobjects.CardBillingSnapshot
}

func TestCreateCardPurchaseSuite(t *testing.T) {
	suite.Run(t, new(CreateCardPurchaseSuite))
}

func (s *CreateCardPurchaseSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.purchases = ifmocks.NewCardPurchaseRepository(s.T())
	s.invoices = ifmocks.NewCardInvoiceRepository(s.T())
	s.cardLookup = ifmocks.NewCardLookup(s.T())
	s.catValidator = ifmocks.NewCategoryValidator(s.T())
	s.publisher = ifmocks.NewCardPurchaseEventPublisher(s.T())

	s.factory.On("CardPurchaseRepository", mock.Anything).Return(s.purchases).Maybe()
	s.factory.On("CardInvoiceRepository", mock.Anything).Return(s.invoices).Maybe()

	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	s.snapshot = snap

	wf := services.NewCardPurchaseWorkflow()
	uow := ucmocks.NewUnitOfWorkCardPurchase(s.T())
	idGen := &testIDGen{}
	s.uc = NewCreateCardPurchase(
		s.factory, s.cardLookup, s.catValidator, &wf,
		s.publisher, uow, idGen, fake.NewProvider(),
	)
}

func (s *CreateCardPurchaseSuite) ctx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(),
		Source: auth.SourceHeader,
	})
}

func (s *CreateCardPurchaseSuite) TestExecute_Unauthorized() {
	raw := input.RawCreateCardPurchase{}
	_, err := s.uc.Execute(context.Background(), raw)
	s.ErrorIs(err, ErrUsecaseUnauthorized)
}

func (s *CreateCardPurchaseSuite) TestExecute_InvalidPurchasedAt() {
	ctx := s.ctx()
	raw := input.RawCreateCardPurchase{
		CardID:            uuid.New(),
		TotalAmountCents:  1000,
		InstallmentsTotal: 1,
		Description:       "test",
		CategoryID:        uuid.New(),
		PurchasedAt:       "invalid-date",
	}
	_, err := s.uc.Execute(ctx, raw)
	s.Error(err)
}

func (s *CreateCardPurchaseSuite) TestExecute_CardNotFound() {
	ctx := s.ctx()
	principal, _ := auth.FromContext(ctx)
	raw := input.RawCreateCardPurchase{
		CardID:            uuid.New(),
		TotalAmountCents:  1000,
		InstallmentsTotal: 1,
		Description:       "test",
		CategoryID:        uuid.New(),
		PurchasedAt:       "2024-01-10T00:00:00Z",
	}
	s.cardLookup.On("GetForUser", mock.Anything, mock.Anything, principal.UserID).
		Return(valueobjects.CardBillingSnapshot{}, interfaces.ErrCardNotFound)

	_, err := s.uc.Execute(ctx, raw)
	s.Error(err)
}

func (s *CreateCardPurchaseSuite) TestExecute_Success_3Installments() {
	ctx := s.ctx()
	principal, _ := auth.FromContext(ctx)

	cardID := uuid.New()
	catID := uuid.New()
	raw := input.RawCreateCardPurchase{
		CardID:            cardID,
		TotalAmountCents:  3000,
		InstallmentsTotal: 3,
		Description:       "Notebook",
		CategoryID:        catID,
		PurchasedAt:       "2024-01-05T00:00:00Z",
	}

	s.cardLookup.On("GetForUser", mock.Anything, mock.Anything, principal.UserID).
		Return(s.snapshot, nil)
	s.catValidator.On("Validate", mock.Anything, catID, (*uuid.UUID)(nil)).
		Return(interfaces.CategorySnapshot{ID: catID, Name: "Eletrônicos"}, nil)

	inv1 := &entities.CardInvoice{}
	*inv1 = entities.NewCardInvoice(uuid.New(), valueobjects.UserIDFromUUID(principal.UserID),
		valueobjects.CardIDFromUUID(cardID), mustRefMonth("2024-01"),
		time.Now(), time.Now(), time.Now())

	s.invoices.On("UpsertByMonth", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(inv1, nil)
	s.invoices.On("ApplyDelta", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	s.purchases.On("Create", mock.Anything, mock.Anything).Return(nil)
	s.purchases.On("ReplaceItems", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.publisher.On("PublishCreated", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	out, err := s.uc.Execute(ctx, raw)
	s.NoError(err)
	s.Equal(int64(3000), out.TotalAmountCents)
	s.Equal(3, out.InstallmentsTotal)
}

func mustRefMonth(s string) valueobjects.RefMonth {
	rm, _ := valueobjects.NewRefMonth(s)
	return rm
}

type testIDGen struct{}

func (g *testIDGen) NewID() string {
	id := uuid.New().String()
	return id
}
