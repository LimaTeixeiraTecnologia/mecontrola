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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type UpdateCardPurchaseSuite struct {
	suite.Suite
	uc           *UpdateCardPurchase
	factory      *ifmocks.RepositoryFactory
	purchases    *ifmocks.CardPurchaseRepository
	invoices     *ifmocks.CardInvoiceRepository
	catValidator *ifmocks.CategoryValidator
	publisher    *ifmocks.CardPurchaseEventPublisher
	snapshot     valueobjects.CardBillingSnapshot
}

func TestUpdateCardPurchaseSuite(t *testing.T) {
	suite.Run(t, new(UpdateCardPurchaseSuite))
}

func (s *UpdateCardPurchaseSuite) SetupTest() {
	s.factory = ifmocks.NewRepositoryFactory(s.T())
	s.purchases = ifmocks.NewCardPurchaseRepository(s.T())
	s.invoices = ifmocks.NewCardInvoiceRepository(s.T())
	s.catValidator = ifmocks.NewCategoryValidator(s.T())
	s.publisher = ifmocks.NewCardPurchaseEventPublisher(s.T())

	s.factory.On("CardPurchaseRepository", mock.Anything).Return(s.purchases).Maybe()
	s.factory.On("CardInvoiceRepository", mock.Anything).Return(s.invoices).Maybe()

	snap, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	s.snapshot = snap

	wf := services.NewCardPurchaseWorkflow()
	uow := ucmocks.NewUnitOfWorkCardPurchase(s.T())
	idGen := &testIDGen{}
	s.uc = NewUpdateCardPurchase(
		s.factory, s.catValidator, &wf,
		s.publisher, uow, idGen, fake.NewProvider(),
	)
}

func (s *UpdateCardPurchaseSuite) ctx() context.Context {
	return auth.WithPrincipal(context.Background(), auth.Principal{
		UserID: uuid.New(),
		Source: auth.SourceHeader,
	})
}

func (s *UpdateCardPurchaseSuite) TestExecute_Unauthorized() {
	_, err := s.uc.Execute(context.Background(), uuid.New(), input.RawUpdateCardPurchase{})
	s.ErrorIs(err, ErrUsecaseUnauthorized)
}

func (s *UpdateCardPurchaseSuite) TestExecute_Cascade12To3_NegativeDeltas() {
	ctx := s.ctx()
	principal, _ := auth.FromContext(ctx)

	userID := principal.UserID
	cardID := uuid.New()
	purchaseID := uuid.New()
	catID := uuid.New()

	amount12, _ := valueobjects.NewMoney(12000)
	inst12, _ := valueobjects.NewInstallmentCount(12)
	desc, _ := valueobjects.NewDescription("Compra 12x")
	catVo, _ := valueobjects.ParseCategoryID(catID.String())

	purchasedAt := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)

	existingPurchase := entities.NewCardPurchase(
		purchaseID,
		valueobjects.UserIDFromUUID(userID),
		valueobjects.CardIDFromUUID(cardID),
		amount12, inst12, desc, catVo,
		option.None[valueobjects.SubcategoryID](),
		"Eletrônicos", "",
		purchasedAt, s.snapshot, time.Now(),
	)

	currentItems := make([]entities.CardInvoiceItem, 12)
	for i := range 12 {
		month := time.Date(2024, time.Month(1+i), 1, 0, 0, 0, 0, time.UTC)
		rm := valueobjects.RefMonthFromTime(month, time.UTC)
		amt, _ := valueobjects.NewMoney(1000)
		currentItems[i] = entities.NewCardInvoiceItem(
			uuid.New(), uuid.New(), purchaseID,
			valueobjects.UserIDFromUUID(userID),
			rm, i+1, amt, time.Now(),
		)
	}

	s.purchases.On("GetByID", mock.Anything, purchaseID, userID).Return(&existingPurchase, nil)

	for i := range 12 {
		month := time.Date(2024, time.Month(1+i), 1, 0, 0, 0, 0, time.UTC)
		rm := valueobjects.RefMonthFromTime(month, time.UTC)
		inv := &entities.CardInvoice{}
		*inv = entities.NewCardInvoice(
			uuid.New(), valueobjects.UserIDFromUUID(userID),
			valueobjects.CardIDFromUUID(cardID), rm,
			month.AddDate(0, 0, 10), month.AddDate(0, 0, 20), time.Now(),
		)
		invItems := []*entities.CardInvoiceItem{&currentItems[i]}
		s.invoices.On("GetByMonth", mock.Anything, userID, cardID, rm).
			Return(inv, invItems, nil).Maybe()
	}

	s.catValidator.On("Validate", mock.Anything, catID, (*uuid.UUID)(nil)).
		Return(interfaces.CategorySnapshot{ID: catID, Name: "Eletrônicos"}, nil)

	s.purchases.On("UpdateWithVersion", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	s.invoices.On("UpsertByMonth", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(func(_ context.Context, uID, cID uuid.UUID, rm valueobjects.RefMonth, _, _ time.Time) *entities.CardInvoice {
			inv := entities.NewCardInvoice(
				uuid.New(), valueobjects.UserIDFromUUID(uID),
				valueobjects.CardIDFromUUID(cID), rm,
				time.Now(), time.Now(), time.Now(),
			)
			return &inv
		}, nil)

	s.purchases.On("ReplaceItems", mock.Anything, purchaseID, mock.Anything).Return(nil)

	s.invoices.On("ApplyDelta", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Maybe()

	s.publisher.On("PublishUpdated", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	raw := input.RawUpdateCardPurchase{
		TotalAmountCents:  3000,
		InstallmentsTotal: 3,
		Description:       "Compra 3x",
		CategoryID:        catID,
		PurchasedAt:       "2024-01-05T00:00:00Z",
		Version:           1,
	}

	out, err := s.uc.Execute(ctx, purchaseID, raw)
	s.NoError(err)
	s.NotEmpty(out.RefMonthsAffected)
	s.Equal(3, out.InstallmentsTotal)
	s.Equal(int64(3000), out.TotalAmountCents)
}
