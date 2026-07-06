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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type CreateTransactionSuite struct {
	suite.Suite
	ctx         context.Context
	userID      uuid.UUID
	factory     *mockInterfaces.RepositoryFactory
	repo        *mockInterfaces.TransactionRepository
	catVal      *mockInterfaces.CategoryValidator
	catGate     *mockInterfaces.CategoryWriteGate
	cardLookup  *mockInterfaces.CardLookup
	invoiceRepo *mockInterfaces.CardInvoiceRepository
	publisher   *mockInterfaces.TransactionEventPublisher
	uow         *uowMocks.UnitOfWorkTransaction
	useCase     *CreateTransaction
}

func TestCreateTransactionSuite(t *testing.T) {
	suite.Run(t, new(CreateTransactionSuite))
}

func (s *CreateTransactionSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.invoiceRepo = mockInterfaces.NewCardInvoiceRepository(s.T())
	s.factory.EXPECT().CardInvoiceRepository(mock.Anything).Return(s.invoiceRepo).Maybe()
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.catGate = mockInterfaces.NewCategoryWriteGate(s.T())
	s.cardLookup = mockInterfaces.NewCardLookup(s.T())
	s.publisher = mockInterfaces.NewTransactionEventPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkTransaction(s.T())
	s.useCase = NewCreateTransaction(
		s.factory, s.uow, s.cardLookup, s.catVal, s.catGate,
		services.TransactionWorkflow{}, s.publisher,
		fake.NewProvider(),
	)
}

func (s *CreateTransactionSuite) TestExecute_Success() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Delivery", ParentName: "Custo Fixo"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(uuid.New(), true, nil).Once()
	s.publisher.EXPECT().
		PublishCreated(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ database.DBTX, evt entities.TransactionCreated) {
			s.Equal(subcategoryID, evt.SubcategoryID)
			s.Equal(catID, evt.CategoryID)
		}).
		Return(nil).
		Once()

	result, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().NoError(err)
	s.Equal("outcome", result.Direction)
	s.Equal("pix", result.PaymentMethod)
	s.Equal(int64(1000), result.AmountCents)
}

func (s *CreateTransactionSuite) TestExecute_OutcomeWithoutSubcategory_ReturnsValidationError() {
	catID := uuid.New()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().ErrorIs(err, ErrTransactionRequiresSubcategory)
}

func (s *CreateTransactionSuite) TestExecute_IncomeWithoutSubcategory_ReturnsValidationError() {
	catID := uuid.New()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   50000,
		Description:   "Salário",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().ErrorIs(err, ErrTransactionRequiresSubcategory)
}

func (s *CreateTransactionSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, input.RawCreateTransaction{})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_ValidationError_Direction() {
	catID := uuid.New()
	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "invalid",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_DocPaymentMethodRejected() {
	catID := uuid.New()
	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "doc",
		AmountCents:   1000,
		Description:   "Pag",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_CategoryValidatorError() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(interfaces.CategorySnapshot{}, interfaces.ErrCategoryNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_RepositoryError() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(uuid.Nil, false, interfaces.ErrTransactionNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "ted",
		AmountCents:   1000,
		Description:   "Renda",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_IdempotencyPublisher() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Test"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(uuid.New(), true, nil).Once()
	s.publisher.EXPECT().PublishCreated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "ted",
		AmountCents:   50000,
		Description:   "Salário",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().NoError(err)
}

func (s *CreateTransactionSuite) TestExecute_Replay_DoesNotPublish() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	canonicalID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Delivery", ParentName: "Custo Fixo"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(canonicalID, false, nil).Once()

	existing := s.newPersistedTransaction(canonicalID, catID)
	s.repo.EXPECT().GetByID(mock.Anything, canonicalID, s.userID).Return(existing, nil).Once()

	result, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:       "outcome",
		PaymentMethod:   "pix",
		AmountCents:     1000,
		Description:     "Mercado",
		CategoryID:      catID,
		SubcategoryID:   &subcategoryID,
		OccurredAt:      time.Now().UTC().Format(time.RFC3339),
		OriginWamid:     "wamid.ABC",
		OriginItemSeq:   0,
		OriginOperation: "create_transaction",
	})

	s.Require().NoError(err)
	s.Equal(canonicalID, result.ID)
	s.publisher.AssertNotCalled(s.T(), "PublishCreated", mock.Anything, mock.Anything, mock.Anything)
}

func (s *CreateTransactionSuite) TestExecute_CreditCard_CreatesInvoiceItems() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	cardID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Eletrônicos", ParentName: "Compras"}
	snapshot, _ := valueobjects.NewCardBillingSnapshot(10, 20)

	rm, _ := valueobjects.NewRefMonth("2026-07")
	userIDVO := valueobjects.UserIDFromUUID(s.userID)
	cardIDVO := valueobjects.CardIDFromUUID(cardID)
	now := time.Now().UTC()
	inv := entities.NewCardInvoice(uuid.New(), userIDVO, cardIDVO, rm, now, now, now)

	s.cardLookup.EXPECT().GetForUser(mock.Anything, cardID, s.userID).Return(snapshot, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(uuid.New(), true, nil).Once()
	s.invoiceRepo.EXPECT().
		UpsertByMonth(mock.Anything, s.userID, cardID, mock.Anything, mock.Anything, mock.Anything).
		Return(&inv, nil).Times(3)
	s.invoiceRepo.EXPECT().
		ApplyDelta(mock.Anything, inv.ID(), mock.AnythingOfType("int64"), int64(1)).
		Return(nil).Times(3)
	s.repo.EXPECT().ReplaceItems(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	s.publisher.EXPECT().
		PublishCreated(mock.Anything, mock.Anything, mock.Anything).
		Run(func(_ context.Context, _ database.DBTX, evt entities.TransactionCreated) {
			s.Len(evt.Installments, 3)
		}).
		Return(nil).Once()

	result, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "credit_card",
		AmountCents:   30000,
		Description:   "Notebook",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		CardID:        &cardID,
		Installments:  3,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().NoError(err)
	s.Equal("credit_card", result.PaymentMethod)
	s.Equal(int64(30000), result.AmountCents)
}

func (s *CreateTransactionSuite) TestExecute_NonCreditCard_DoesNotLookupCard() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Salário"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(uuid.New(), true, nil).Once()
	s.publisher.EXPECT().PublishCreated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   50000,
		Description:   "Renda",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().NoError(err)
	s.cardLookup.AssertNotCalled(s.T(), "GetForUser", mock.Anything, mock.Anything, mock.Anything)
	s.invoiceRepo.AssertNotCalled(s.T(), "UpsertByMonth", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func (s *CreateTransactionSuite) TestExecute_CreditCard_LookupError() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	cardID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Eletrônicos", ParentName: "Compras"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.cardLookup.EXPECT().GetForUser(mock.Anything, cardID, s.userID).Return(valueobjects.CardBillingSnapshot{}, interfaces.ErrCardNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "credit_card",
		AmountCents:   30000,
		Description:   "Notebook",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		CardID:        &cardID,
		Installments:  3,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().Error(err)
	s.repo.AssertNotCalled(s.T(), "Create", mock.Anything, mock.Anything)
}

func (s *CreateTransactionSuite) TestExecute_GateBlocks_ReturnsError() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Despesa", Kind: "expense"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).
		Return(valueobjects.CategoryWriteEvidence{}, valueobjects.ErrCategoryKindDirectionMismatch).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().ErrorIs(err, valueobjects.ErrCategoryKindDirectionMismatch)
	s.repo.AssertNotCalled(s.T(), "Create", mock.Anything, mock.Anything)
}

func (s *CreateTransactionSuite) TestExecute_GateBlocks_DeprecatedCategory() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Despesa", Kind: "expense"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).
		Return(valueobjects.CategoryWriteEvidence{}, valueobjects.ErrCategoryDeprecated).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().ErrorIs(err, valueobjects.ErrCategoryDeprecated)
}

func (s *CreateTransactionSuite) TestExecute_GateBlocks_VersionDrift() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Despesa", Kind: "expense"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).
		Return(valueobjects.CategoryWriteEvidence{}, valueobjects.ErrCategoryVersionChanged).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().ErrorIs(err, valueobjects.ErrCategoryVersionChanged)
}

func (s *CreateTransactionSuite) TestExecute_GateBlocks_RootWithoutLeaf() {
	catID := uuid.New()
	subcategoryID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Despesa", Kind: "expense"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).
		Return(valueobjects.CategoryWriteEvidence{}, valueobjects.ErrCategoryRootWithoutLeaf).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().ErrorIs(err, valueobjects.ErrCategoryRootWithoutLeaf)
}

func (s *CreateTransactionSuite) newPersistedTransaction(id, catID uuid.UUID) *entities.Transaction {
	amount, _ := valueobjects.NewMoney(1000)
	desc, _ := valueobjects.NewDescription("Mercado")
	rm, _ := valueobjects.NewRefMonth("2026-07")
	now := time.Now().UTC()
	tx := entities.NewTransaction(
		id,
		valueobjects.UserIDFromUUID(s.userID),
		valueobjects.DirectionOutcome,
		valueobjects.PaymentMethodPix,
		amount, desc,
		valueobjects.CategoryIDFromUUID(catID),
		option.None[valueobjects.SubcategoryID](),
		"Custo Fixo", "Delivery",
		valueobjects.CategoryWriteEvidence{},
		rm, now, now,
	)
	return &tx
}
