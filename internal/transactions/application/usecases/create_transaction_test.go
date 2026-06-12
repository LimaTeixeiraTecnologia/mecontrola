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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/services"
)

type CreateTransactionSuite struct {
	suite.Suite
	ctx       context.Context
	userID    uuid.UUID
	factory   *mockInterfaces.RepositoryFactory
	repo      *mockInterfaces.TransactionRepository
	catVal    *mockInterfaces.CategoryValidator
	publisher *mockInterfaces.TransactionEventPublisher
	uow       *uowMocks.UnitOfWorkTransaction
	useCase   *usecases.CreateTransaction
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
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.publisher = mockInterfaces.NewTransactionEventPublisher(s.T())
	s.uow = uowMocks.NewUnitOfWorkTransaction(s.T())
	s.useCase = usecases.NewCreateTransaction(
		s.factory, s.uow, s.catVal,
		services.TransactionWorkflow{}, s.publisher,
		noop.NewProvider(),
	)
}

func (s *CreateTransactionSuite) TestExecute_Success() {
	catID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Custo Fixo"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	s.publisher.EXPECT().PublishCreated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().NoError(err)
	s.Equal("outcome", result.Direction)
	s.Equal("pix", result.PaymentMethod)
	s.Equal(int64(1000), result.AmountCents)
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
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(interfaces.CategorySnapshot{}, interfaces.ErrCategoryNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Mercado",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_RepositoryError() {
	catID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Cat"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(interfaces.ErrTransactionNotFound).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "ted",
		AmountCents:   1000,
		Description:   "Renda",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateTransactionSuite) TestExecute_IdempotencyPublisher() {
	catID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Test"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()
	s.publisher.EXPECT().PublishCreated(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateTransaction{
		Direction:     "income",
		PaymentMethod: "ted",
		AmountCents:   50000,
		Description:   "Salário",
		CategoryID:    catID,
		OccurredAt:    time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().NoError(err)
}
