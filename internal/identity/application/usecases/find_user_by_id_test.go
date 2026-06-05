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
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type FindUserByIDSuite struct {
	suite.Suite
	mgr         *mocks.FakeManager
	factoryMock *mocks.RepositoryFactory
	repoMock    *mocks.UserRepository
	uc          *usecases.FindUserByID
}

func TestFindUserByID(t *testing.T) {
	suite.Run(t, new(FindUserByIDSuite))
}

func (s *FindUserByIDSuite) SetupTest() {
	s.mgr = mocks.NewFakeManager()
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.repoMock = mocks.NewUserRepository(s.T())
	s.uc = usecases.NewFindUserByID(s.mgr, s.factoryMock, noop.NewProvider())
}

func (s *FindUserByIDSuite) TestEncontrado() {
	wa, _ := valueobjects.NewWhatsAppNumber("+5511987654321")
	user := entities.New(wa, entities.WithDisplayName("Test User"))

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByID", mock.Anything, user.ID()).Return(user, nil)

	out, err := s.uc.Execute(context.Background(), input.FindUserByID{ID: user.ID()})
	s.Require().NoError(err)
	s.Equal(user.ID(), out.ID)
}

func (s *FindUserByIDSuite) TestErrUserNotFound() {
	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByID", mock.Anything, "non-existent").Return(entities.User{}, application.ErrUserNotFound)

	_, err := s.uc.Execute(context.Background(), input.FindUserByID{ID: "non-existent"})
	s.Require().Error(err)
	s.True(errors.Is(err, application.ErrUserNotFound))
}

func (s *FindUserByIDSuite) TestErroDeIO() {
	ioErr := errors.New("db unavailable")
	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByID", mock.Anything, "some-id").Return(entities.User{}, ioErr)

	_, err := s.uc.Execute(context.Background(), input.FindUserByID{ID: "some-id"})
	s.Require().Error(err)
	s.True(errors.Is(err, ioErr))
}
