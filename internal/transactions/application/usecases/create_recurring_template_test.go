package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
)

type CreateRecurringTemplateSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.RecurringTemplateRepository
	catVal  *mockInterfaces.CategoryValidator
	uow     *uowMocks.UnitOfWorkRecurringTemplate
	useCase *CreateRecurringTemplate
}

func TestCreateRecurringTemplateSuite(t *testing.T) {
	suite.Run(t, new(CreateRecurringTemplateSuite))
}

func (s *CreateRecurringTemplateSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.repo).Maybe()
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.uow = uowMocks.NewUnitOfWorkRecurringTemplate(s.T())
	s.useCase = NewCreateRecurringTemplate(
		s.factory, s.uow, s.catVal, fake.NewProvider(),
	)
}

func (s *CreateRecurringTemplateSuite) TestExecute_Success() {
	catID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Receita"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(nil).Once()

	result, err := s.useCase.Execute(s.ctx, input.RawCreateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   300000,
		Description:   "Salário",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	})

	s.Require().NoError(err)
	s.Equal("income", result.Direction)
	s.Equal("pix", result.PaymentMethod)
	s.Equal(int64(300000), result.AmountCents)
	s.Equal(5, result.DayOfMonth)
}

func (s *CreateRecurringTemplateSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, input.RawCreateRecurringTemplate{})
	s.Require().Error(err)
}

func (s *CreateRecurringTemplateSuite) TestExecute_CreditCard_RequiresCardID() {
	catID := uuid.New()
	_, err := s.useCase.Execute(s.ctx, input.RawCreateRecurringTemplate{
		Direction:     "outcome",
		PaymentMethod: "credit_card",
		AmountCents:   50000,
		Description:   "Assinatura",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    10,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateRecurringTemplateSuite) TestExecute_InvalidStartedAt() {
	catID := uuid.New()
	_, err := s.useCase.Execute(s.ctx, input.RawCreateRecurringTemplate{
		Direction:     "outcome",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Test",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    1,
		StartedAt:     "not-a-date",
	})
	s.Require().Error(err)
}

func (s *CreateRecurringTemplateSuite) TestExecute_CategoryValidationError() {
	catID := uuid.New()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).
		Return(interfaces.CategorySnapshot{}, errors.New("category not found")).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   300000,
		Description:   "Salário",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}

func (s *CreateRecurringTemplateSuite) TestExecute_CreateRepoError() {
	catID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Receita"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().Create(mock.Anything, mock.Anything).Return(errors.New("db error")).Once()

	_, err := s.useCase.Execute(s.ctx, input.RawCreateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   300000,
		Description:   "Salário",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
	})
	s.Require().Error(err)
}
