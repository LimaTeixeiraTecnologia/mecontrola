package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/auth"
	mockInterfaces "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/interfaces/mocks"
	uowMocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/transactions/application/usecases/mocks"
)

type DeleteRecurringTemplateSuite struct {
	suite.Suite
	ctx        context.Context
	userID     uuid.UUID
	templateID uuid.UUID
	factory    *mockInterfaces.RepositoryFactory
	repo       *mockInterfaces.RecurringTemplateRepository
	uow        *uowMocks.UnitOfWorkVoid
	useCase    *DeleteRecurringTemplate
}

func TestDeleteRecurringTemplateSuite(t *testing.T) {
	suite.Run(t, new(DeleteRecurringTemplateSuite))
}

func (s *DeleteRecurringTemplateSuite) SetupTest() {
	s.userID = uuid.New()
	s.templateID = uuid.New()
	s.ctx = auth.WithPrincipal(context.Background(), auth.Principal{UserID: s.userID, Source: auth.SourceHeader})
	s.factory = mockInterfaces.NewRepositoryFactory(s.T())
	s.repo = mockInterfaces.NewRecurringTemplateRepository(s.T())
	s.factory.EXPECT().RecurringTemplateRepository(mock.Anything).Return(s.repo).Maybe()
	s.uow = uowMocks.NewUnitOfWorkVoid(s.T())
	s.useCase = NewDeleteRecurringTemplate(
		s.factory, s.uow, fake.NewProvider(),
	)
}

func (s *DeleteRecurringTemplateSuite) TestExecute_Success() {
	s.repo.EXPECT().SoftDelete(mock.Anything, s.templateID, s.userID, int64(1), mock.Anything).Return(nil).Once()

	err := s.useCase.Execute(s.ctx, s.templateID.String(), 1)
	s.Require().NoError(err)
}

func (s *DeleteRecurringTemplateSuite) TestExecute_Unauthorized() {
	ctx := context.Background()
	err := s.useCase.Execute(ctx, s.templateID.String(), 1)
	s.Require().Error(err)
}

func (s *DeleteRecurringTemplateSuite) TestExecute_InvalidID() {
	err := s.useCase.Execute(s.ctx, "invalid-uuid", 1)
	s.Require().Error(err)
}

func (s *DeleteRecurringTemplateSuite) TestExecute_SoftDeleteError() {
	s.repo.EXPECT().SoftDelete(mock.Anything, s.templateID, s.userID, int64(1), mock.Anything).
		Return(errors.New("version conflict")).Once()

	err := s.useCase.Execute(s.ctx, s.templateID.String(), 1)
	s.Require().Error(err)
}
