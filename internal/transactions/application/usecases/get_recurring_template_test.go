package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type GetRecurringTemplateSuite struct {
	suite.Suite
	ctx        context.Context
	userID     uuid.UUID
	templateID uuid.UUID
	factory    *mockInterfaces.RepositoryFactory
	repo       *mockInterfaces.RecurringTemplateRepository
	uow        *uowMocks.UnitOfWorkOutputRecurringTemplate
	useCase    *usecases.GetRecurringTemplate
}

func TestGetRecurringTemplateSuite(t *testing.T) {
	suite.Run(t, new(GetRecurringTemplateSuite))
}

func (s *GetRecurringTemplateSuite) SetupTest() {
	s.userID = uuid.New()
	s.templateID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkOutputRecurringTemplate(s.T())
	s.useCase = usecases.NewGetRecurringTemplate(s.factory, s.uow, noop.NewProvider())
}

func (s *GetRecurringTemplateSuite) buildTemplate() *entities.RecurringTemplate {
	dir := valueobjects.DirectionIncome
	pm := valueobjects.PaymentMethodPix
	amount, _ := valueobjects.NewMoney(300000)
	desc, _ := valueobjects.NewDescription("Salário")
	catID := valueobjects.CategoryIDFromUUID(uuid.New())
	freq := valueobjects.FrequencyMonthly
	dom, _ := valueobjects.NewDayOfMonth(5)
	inst, _ := valueobjects.NewInstallmentCount(1)
	now := time.Now().UTC()
	t := entities.NewRecurringTemplate(
		s.templateID, valueobjects.UserIDFromUUID(s.userID),
		dir, pm, option.None[valueobjects.CardID](),
		amount, desc, catID, option.None[valueobjects.SubcategoryID](),
		"Receita", "", freq, dom, inst,
		now, option.None[time.Time](), now,
	)
	return &t
}

func (s *GetRecurringTemplateSuite) TestExecute_Success() {
	existing := s.buildTemplate()
	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).Return(existing, nil).Once()

	result, err := s.useCase.Execute(s.ctx, s.templateID.String())
	s.Require().NoError(err)
	s.Equal(s.templateID, result.ID)
}

func (s *GetRecurringTemplateSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, s.templateID.String())
	s.Require().Error(err)
}

func (s *GetRecurringTemplateSuite) TestExecute_InvalidID() {
	_, err := s.useCase.Execute(s.ctx, "not-a-uuid")
	s.Require().Error(err)
}

func (s *GetRecurringTemplateSuite) TestExecute_NotFound() {
	s.repo.EXPECT().GetByID(mock.Anything, s.templateID, s.userID).
		Return(nil, errors.New("not found")).Once()

	_, err := s.useCase.Execute(s.ctx, s.templateID.String())
	s.Require().Error(err)
}
