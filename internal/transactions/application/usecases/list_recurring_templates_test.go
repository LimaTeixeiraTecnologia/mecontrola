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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/option"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/domain/valueobjects"
)

type ListRecurringTemplatesSuite struct {
	suite.Suite
	ctx     context.Context
	userID  uuid.UUID
	factory *mockInterfaces.RepositoryFactory
	repo    *mockInterfaces.RecurringTemplateRepository
	uow     *uowMocks.UnitOfWorkRecurringTemplatePage
	useCase *usecases.ListRecurringTemplates
}

func TestListRecurringTemplatesSuite(t *testing.T) {
	suite.Run(t, new(ListRecurringTemplatesSuite))
}

func (s *ListRecurringTemplatesSuite) SetupTest() {
	s.userID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkRecurringTemplatePage(s.T())
	s.useCase = usecases.NewListRecurringTemplates(s.factory, s.uow, noop.NewProvider())
}

func (s *ListRecurringTemplatesSuite) buildTemplate() *entities.RecurringTemplate {
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
		uuid.New(), valueobjects.UserIDFromUUID(s.userID),
		dir, pm, option.None[valueobjects.CardID](),
		amount, desc, catID, option.None[valueobjects.SubcategoryID](),
		"Receita", "", freq, dom, inst,
		now, option.None[time.Time](), now,
	)
	return &t
}

func (s *ListRecurringTemplatesSuite) TestExecute_Success() {
	templates := []*entities.RecurringTemplate{s.buildTemplate(), s.buildTemplate()}
	s.repo.EXPECT().List(mock.Anything, s.userID, true, interfaces.Cursor{}, 50).
		Return(templates, interfaces.Cursor{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, true, "", 50)
	s.Require().NoError(err)
	s.Len(result.Templates, 2)
	s.Empty(result.NextCursor)
}

func (s *ListRecurringTemplatesSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	_, err := s.useCase.Execute(ctx, true, "", 50)
	s.Require().Error(err)
}

func (s *ListRecurringTemplatesSuite) TestExecute_DefaultLimit() {
	s.repo.EXPECT().List(mock.Anything, s.userID, false, interfaces.Cursor{}, 50).
		Return(nil, interfaces.Cursor{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, false, "", 0)
	s.Require().NoError(err)
	s.Empty(result.Templates)
}

func (s *ListRecurringTemplatesSuite) TestExecute_CapLimit() {
	s.repo.EXPECT().List(mock.Anything, s.userID, true, interfaces.Cursor{}, 200).
		Return(nil, interfaces.Cursor{}, nil).Once()

	result, err := s.useCase.Execute(s.ctx, true, "", 999)
	s.Require().NoError(err)
	s.Empty(result.Templates)
}

func (s *ListRecurringTemplatesSuite) TestExecute_RepoError() {
	s.repo.EXPECT().List(mock.Anything, s.userID, true, interfaces.Cursor{}, 50).
		Return(nil, interfaces.Cursor{}, errors.New("db error")).Once()

	_, err := s.useCase.Execute(s.ctx, true, "", 0)
	s.Require().Error(err)
}
