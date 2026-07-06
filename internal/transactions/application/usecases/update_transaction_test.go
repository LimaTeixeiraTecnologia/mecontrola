package usecases

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type UpdateTransactionSuite struct {
	suite.Suite
	ctx         context.Context
	userID      uuid.UUID
	factory     *mockInterfaces.RepositoryFactory
	repo        *mockInterfaces.TransactionRepository
	invoiceRepo *mockInterfaces.CardInvoiceRepository
	catVal      *mockInterfaces.CategoryValidator
	catGate     *mockInterfaces.CategoryWriteGate
	publisher   *mockInterfaces.TransactionEventPublisher
	uow         *uowMocks.UnitOfWorkTransaction
	useCase     *UpdateTransaction
}

func TestUpdateTransactionSuite(t *testing.T) {
	suite.Run(t, new(UpdateTransactionSuite))
}

func (s *UpdateTransactionSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewTransactionRepository(s.T())
	s.factory.EXPECT().TransactionRepository(mock.Anything).Return(s.repo).Maybe()
	s.invoiceRepo = mockInterfaces.NewCardInvoiceRepository(s.T())
	s.factory.EXPECT().CardInvoiceRepository(mock.Anything).Return(s.invoiceRepo).Maybe()
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.catGate = mockInterfaces.NewCategoryWriteGate(s.T())
	s.publisher = mockInterfaces.NewTransactionEventPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkTransaction(s.T())
	s.useCase = NewUpdateTransaction(
		s.factory, s.uow, s.catVal, s.catGate,
		services.TransactionWorkflow{}, s.publisher,
		fake.NewProvider(),
	)
}

func (s *UpdateTransactionSuite) makeExistingTransaction(txID uuid.UUID) *entities.Transaction {
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
		"Custo Fixo", "",
		valueobjects.CategoryWriteEvidence{},
		rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	return &tx
}

func (s *UpdateTransactionSuite) TestExecute_Success() {
	txID := uuid.New()
	catID := uuid.New()
	subID := uuid.New()
	existing := s.makeExistingTransaction(txID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Metas"}

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(nil).Once()
	s.publisher.EXPECT().PublishUpdated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   2000,
		Description:   "Supermercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})

	s.Require().NoError(err)
	s.Equal("outcome", result.Direction)
	s.Equal(int64(2000), result.AmountCents)
}

func (s *UpdateTransactionSuite) TestExecute_RefMonthChange_TwoCompetencias() {
	txID := uuid.New()
	catID := uuid.New()

	userID := valueobjects.UserIDFromUUID(s.userID)
	dir := valueobjects.DirectionOutcome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(1000)
	desc, _ := valueobjects.NewDescription("Mercado")
	catIDVO := valueobjects.CategoryIDFromUUID(catID)
	rm, _ := valueobjects.NewRefMonth("2026-05")
	now := time.Now().UTC()

	existing := entities.Reconstitute(
		txID, userID, dir, pm, amount, desc, catIDVO,
		option.None[valueobjects.SubcategoryID](),
		"Cat", "", valueobjects.CategoryWriteEvidence{}, rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)

	subID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(&existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(nil).Once()
	s.publisher.EXPECT().PublishUpdated(mock.Anything, mock.Anything, mock.Anything).Run(func(_ context.Context, _ database.DBTX, evt entities.TransactionUpdated) {
		s.Len(evt.RefMonthsAffected, 2, "deve conter old e new ref_month")
	}).Return(nil).Once()

	juneDate := time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC)
	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1500,
		Description:   "Supermercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    juneDate.Format(time.RFC3339),
		Version:       1,
	})
	s.Require().NoError(err)
}

func (s *UpdateTransactionSuite) TestExecute_OutcomeWithoutSubcategory_ReturnsValidationError() {
	txID := uuid.New()
	catID := uuid.New()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})

	s.Require().ErrorIs(err, ErrTransactionRequiresSubcategory)
}

func (s *UpdateTransactionSuite) TestExecute_IncomeWithoutSubcategory_ReturnsValidationError() {
	txID := uuid.New()
	catID := uuid.New()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Salário",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})

	s.Require().ErrorIs(err, ErrTransactionRequiresSubcategory)
}

func (s *UpdateTransactionSuite) TestExecute_CreditCard_RecomposesDeltas() {
	txID := uuid.New()
	catID := uuid.New()
	subcategoryID := uuid.New()
	cardID := uuid.New()
	userIDVO := valueobjects.UserIDFromUUID(s.userID)
	cardIDVO := valueobjects.CardIDFromUUID(cardID)
	amount, _ := valueobjects.NewMoney(20000)
	desc, _ := valueobjects.NewDescription("Notebook")
	catIDVO := valueobjects.CategoryIDFromUUID(catID)
	rm, _ := valueobjects.NewRefMonth("2026-07")
	snapshot, _ := valueobjects.NewCardBillingSnapshot(10, 20)
	installments, _ := valueobjects.NewInstallmentCount(2)
	now := time.Now().UTC()

	existing := entities.Reconstitute(
		txID, userIDVO, valueobjects.DirectionOutcome, valueobjects.PaymentMethodCreditCard,
		amount, desc, catIDVO, option.Some(valueobjects.SubcategoryIDFromUUID(subcategoryID)),
		"Compras", "Eletrônicos", valueobjects.CategoryWriteEvidence{}, rm, now, option.None[valueobjects.CardID](), option.None[valueobjects.InstallmentCount](), option.None[valueobjects.CardBillingSnapshot](), 1, nil, now, now,
	)
	existing.SetCardBilling(cardIDVO, installments, snapshot)

	oldAmount, _ := valueobjects.NewMoney(10000)
	oldItem := entities.NewCardInvoiceItem(uuid.New(), uuid.New(), txID, userIDVO, rm, 1, oldAmount, now)
	inv := entities.NewCardInvoice(uuid.New(), userIDVO, cardIDVO, rm, now, now, now)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Eletrônicos", ParentName: "Compras"}

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(&existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return([]*entities.CardInvoiceItem{&oldItem}, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subcategoryID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(nil).Once()
	s.invoiceRepo.EXPECT().
		UpsertByMonth(mock.Anything, s.userID, cardID, mock.Anything, mock.Anything, mock.Anything).
		Return(&inv, nil).Times(2)
	s.repo.EXPECT().ReplaceItems(mock.Anything, txID, mock.Anything).Return(nil).Once()
	s.invoiceRepo.EXPECT().GetByMonth(mock.Anything, s.userID, cardID, mock.Anything).Return(&inv, nil, nil).Maybe()
	s.invoiceRepo.EXPECT().ApplyDelta(mock.Anything, inv.ID(), mock.AnythingOfType("int64"), int64(1)).Return(nil).Maybe()
	s.publisher.EXPECT().PublishUpdated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "credit_card",
		AmountCents:   20000,
		Description:   "Notebook",
		CategoryID:    catID,
		SubcategoryID: &subcategoryID,
		CardID:        &cardID,
		Installments:  2,
		OccurredAt:    now.Format(time.RFC3339),
		Version:       1,
	})

	s.Require().NoError(err)
}

func (s *UpdateTransactionSuite) TestExecute_Unauthorized() {
	_, err := s.useCase.Execute(context.Background(), uuid.NewString(), input.RawUpdateTransaction{})
	s.Require().Error(err)
}

func (s *UpdateTransactionSuite) TestExecute_GetByIDError() {
	txID := uuid.New()
	catID := uuid.New()
	subID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(catSnap, nil).Once()
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(nil, interfaces.ErrTransactionNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateTransactionSuite) TestExecute_CatValidatorError() {
	txID := uuid.New()
	catID := uuid.New()
	subID := uuid.New()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(interfaces.CategorySnapshot{}, interfaces.ErrCategoryNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateTransactionSuite) TestExecute_GateBlocks_KindMismatch() {
	txID := uuid.New()
	catID := uuid.New()
	subID := uuid.New()
	existing := s.makeExistingTransaction(txID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Despesa", Kind: "expense"}
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).
		Return(valueobjects.CategoryWriteEvidence{}, valueobjects.ErrCategoryKindDirectionMismatch).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})

	s.Require().ErrorIs(err, valueobjects.ErrCategoryKindDirectionMismatch)
}

func (s *UpdateTransactionSuite) TestExecute_UpdateWithoutCategoryChange_GateCalledAndEvidenceUpdated() {
	txID := uuid.New()
	catID := uuid.New()
	subID := uuid.New()
	existing := s.makeExistingTransaction(txID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Metas"}

	var capturedInput interfaces.CategoryWriteGateInput
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).
		Run(func(_ context.Context, in interfaces.CategoryWriteGateInput) {
			capturedInput = in
		}).
		Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(nil).Once()
	s.publisher.EXPECT().PublishUpdated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})

	s.Require().NoError(err)
	s.Equal(catID, capturedInput.RootCategoryID)
	s.Equal(subID, capturedInput.SubcategoryID)
	s.Equal(valueobjects.CategoryDecisionSourceManualCanonicalID, capturedInput.Source)
}

func (s *UpdateTransactionSuite) TestExecute_VersionConflict() {
	txID := uuid.New()
	catID := uuid.New()
	subID := uuid.New()
	existing := s.makeExistingTransaction(txID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.repo.EXPECT().GetItemsByTransactionID(mock.Anything, txID).Return(nil, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, &subID).Return(catSnap, nil).Once()
	s.catGate.EXPECT().Approve(mock.Anything, mock.Anything).Return(valueobjects.CategoryWriteEvidence{}, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(interfaces.ErrTransactionVersionConflict).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		SubcategoryID: &subID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}
