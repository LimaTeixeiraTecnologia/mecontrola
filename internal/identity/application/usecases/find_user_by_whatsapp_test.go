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

type FindUserByWhatsAppSuite struct {
	suite.Suite
	mgr         *mocks.FakeManager
	factoryMock *mocks.RepositoryFactory
	repoMock    *mocks.UserRepository
	uc          *usecases.FindUserByWhatsApp
}

func TestFindUserByWhatsApp(t *testing.T) {
	suite.Run(t, new(FindUserByWhatsAppSuite))
}

func (s *FindUserByWhatsAppSuite) SetupTest() {
	s.mgr = mocks.NewFakeManager()
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.repoMock = mocks.NewUserRepository(s.T())
	s.uc = usecases.NewFindUserByWhatsApp(s.mgr, s.factoryMock, noop.NewProvider())
}

func (s *FindUserByWhatsAppSuite) validWhatsApp() valueobjects.WhatsAppNumber {
	wa, err := valueobjects.NewWhatsAppNumber("+5511987654321")
	s.Require().NoError(err)
	return wa
}

func (s *FindUserByWhatsAppSuite) TestEncontrado() {
	wa := s.validWhatsApp()
	user := entities.New(wa)

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(user, nil)

	out, err := s.uc.Execute(context.Background(), input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"})
	s.Require().NoError(err)
	s.Equal(user.ID(), out.ID)
}

func (s *FindUserByWhatsAppSuite) TestErrUserNotFound() {
	wa := s.validWhatsApp()

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)

	_, err := s.uc.Execute(context.Background(), input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"})
	s.Require().Error(err)
	s.True(errors.Is(err, application.ErrUserNotFound))
}

func (s *FindUserByWhatsAppSuite) TestErroDeIO() {
	wa := s.validWhatsApp()
	ioErr := errors.New("db unavailable")

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, ioErr)

	_, err := s.uc.Execute(context.Background(), input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"})
	s.Require().Error(err)
	s.True(errors.Is(err, ioErr))
}

func (s *FindUserByWhatsAppSuite) TestParseWhatsAppInvalid() {
	_, err := s.uc.Execute(context.Background(), input.FindUserByWhatsApp{WhatsAppNumber: "not-a-number"})
	s.Require().Error(err)
}
