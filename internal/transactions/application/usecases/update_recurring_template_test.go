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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type UpdateRecurringTemplateSuite struct {
	suite.Suite
	ctx        context.Context
	userID     uuid.UUID
	templateID uuid.UUID
	factory    *mockInterfaces.RepositoryFactory
	repo       *mockInterfaces.RecurringTemplateRepository
	catVal     *mockInterfaces.CategoryValidator
	uow        *uowMocks.UnitOfWorkRecurringTemplate
	useCase    *UpdateRecurringTemplate
}

func TestUpdateRecurringTemplateSuite(t *testing.T) {
	suite.Run(t, new(UpdateRecurringTemplateSuite))
}

func (s *UpdateRecurringTemplateSuite) SetupTest() {
	s.userID = uuid.New()
	s.templateID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.repo).Maybe()
	s.catVal = mockInterfaces.NewCategoryValidator(s.T())
	s.uow = uowMocks.NewUnitOfWorkRecurringTemplate(s.T())
	s.useCase = NewUpdateRecurringTemplate(
		s.factory, s.uow, s.catVal, fake.NewProvider(),
	)
}

func (s *UpdateRecurringTemplateSuite) buildTemplate(catID uuid.UUID) *entities.RecurringTemplate {
	dir := valueobjects.DirectionIncome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(300000)
	desc, _ := valueobjects.NewDescription("Salário")
	catIDVo := valueobjects.CategoryIDFromUUID(catID)
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(5)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		s.templateID,
		valueobjects.UserIDFromUUID(s.userID),
		dir, pm,
		option.None[valueobjects.CardID](),
		amount, desc, catIDVo,
		option.None[valueobjects.SubcategoryID](),
		"Receita", "",
		freq, dom, inst,
		now, option.None[time.Time](), now,
	)
	return &t
}

func (s *UpdateRecurringTemplateSuite) TestExecute_Success() {
	catID := uuid.New()
	existing := s.buildTemplate(catID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Receita"}

	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).Return(existing, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, mock.AnythingOfType("int64")).Return(nil).Once()

	result, err := s.useCase.Execute(s.ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   350000,
		Description:   "Salário Atualizado",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})

	s.Require().NoError(err)
	s.Equal(int64(350000), result.AmountCents)
}

func (s *UpdateRecurringTemplateSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{})
	s.Require().Error(err)
}

func (s *UpdateRecurringTemplateSuite) TestExecute_InvalidStartedAt() {
	catID := uuid.New()
	_, err := s.useCase.Execute(s.ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   1000,
		Description:   "Test",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     "invalid-date",
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateRecurringTemplateSuite) TestExecute_CommandError() {
	catID := uuid.New()
	_, err := s.useCase.Execute(s.ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "credit_card",
		AmountCents:   1000,
		Description:   "Test",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateRecurringTemplateSuite) TestExecute_NotFound() {
	catID := uuid.New()
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Receita"}
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).
		Return(nil, errors.New("not found")).Once()

	_, err := s.useCase.Execute(s.ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   350000,
		Description:   "Salário Atualizado",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateRecurringTemplateSuite) TestExecute_CategoryError() {
	catID := uuid.New()

	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).
		Return(interfaces.CategorySnapshot{}, errors.New("cat not found")).Once()

	_, err := s.useCase.Execute(s.ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   350000,
		Description:   "Salário Atualizado",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}

func (s *UpdateRecurringTemplateSuite) TestExecute_UpdateVersionConflict() {
	catID := uuid.New()
	existing := s.buildTemplate(catID)
	catSnap := interfaces.CategorySnapshot{ID: catID, Name: "Receita"}

	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).Return(existing, nil).Once()
	s.catVal.EXPECT().Validate(mock.Anything, catID, (*uuid.UUID)(nil)).Return(catSnap, nil).Once()
	s.repo.EXPECT().UpdateWithVersion(mock.Anything, mock.Anything, mock.AnythingOfType("int64")).
		Return(errors.New("version conflict")).Once()

	_, err := s.useCase.Execute(s.ctx, s.templateID.String(), input.RawUpdateRecurringTemplate{
		Direction:     "income",
		PaymentMethod: "pix",
		AmountCents:   350000,
		Description:   "Salário Atualizado",
		CategoryID:    catID,
		Frequency:     "monthly",
		DayOfMonth:    5,
		StartedAt:     time.Now().UTC().Format(time.RFC3339),
		Version:       1,
	})
	s.Require().Error(err)
}
