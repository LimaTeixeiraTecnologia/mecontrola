package usecases_test

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
)

type MarkUserDeletedSuite struct {
	suite.Suite
	uowMock     *mocks.UnitOfWorkVoid
	factoryMock *mocks.RepositoryFactory
	repoMock    *mocks.UserRepository
	uc          *usecases.MarkUserDeleted
}

func TestMarkUserDeleted(t *testing.T) {
	suite.Run(t, new(MarkUserDeletedSuite))
}

func (s *MarkUserDeletedSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkVoid(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.repoMock = mocks.NewUserRepository(s.T())
	s.uc = usecases.NewMarkUserDeleted(s.uowMock, s.factoryMock, noop.NewProvider())
}

func (s *MarkUserDeletedSuite) TestOk() {
	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("MarkDeleted", mock.Anything, "user-id-1", mock.Anything).Return(nil)

	err := s.uc.Execute(context.Background(), input.MarkUserDeleted{ID: "user-id-1"})
	s.Require().NoError(err)
}

func (s *MarkUserDeletedSuite) TestErrUserNotFoundPropagado() {
	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("MarkDeleted", mock.Anything, "not-found", mock.Anything).Return(application.ErrUserNotFound)

	err := s.uc.Execute(context.Background(), input.MarkUserDeleted{ID: "not-found"})
	s.Require().Error(err)
	s.True(errors.Is(err, application.ErrUserNotFound))
}
