package usecases_test

import (
	"context"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type UpdateTransactionSuite struct {
	suite.Suite
	ctx       context.Context
	userID    uuid.UUID
	factory   *mockInterfaces.RepositoryFactory
	repo      *mockInterfaces.TransactionRepository
	catVal    *mockInterfaces.CategoryValidator
	publisher *mockInterfaces.TransactionEventPublisher
	uow       *uowMocks.UnitOfWorkTransaction
	useCase   *usecases.UpdateTransaction
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
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.publisher = mockInterfaces.NewTransactionEventPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkTransaction(s.T())
	s.useCase = usecases.NewUpdateTransaction(
		s.factory, s.uow, s.catVal,
		services.TransactionWorkflow{}, s.publisher,
		noop.NewProvider(),
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
		rm, now, 1, nil, now, now,
	)
	return &tx
}

func (s *UpdateTransactionSuite) TestExecute_Success() {
	txID := uuid.New()
	catID := uuid.New()
	existing := s.makeExistingTransaction(txID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Metas"}

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(nil).Once()
	s.publisher.EXPECT().PublishUpdated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   2000,
		Description:   "Supermercado",
		CategoryID:    catID,
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
		"Cat", "", rm, now, 1, nil, now, now,
	)

	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(&existing, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
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
		OccurredAt:    juneDate.Format(time.RFC3339),
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
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(nil, interfaces.ErrTransactionNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateTransactionSuite) TestExecute_CatValidatorError() {
	txID := uuid.New()
	catID := uuid.New()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(interfaces.CategorySnapshot{}, interfaces.ErrCategoryNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateTransactionSuite) TestExecute_VersionConflict() {
	txID := uuid.New()
	catID := uuid.New()
	existing := s.makeExistingTransaction(txID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}

	s.repo.EXPECT().GetByID(mock.Anything, txID, s.userID).Return(existing, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, int64(1)).Return(interfaces.ErrTransactionVersionConflict).Once()

	_, err := s.useCase.Execute(s.ctx, txID.String(), input.RawUpdateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}
