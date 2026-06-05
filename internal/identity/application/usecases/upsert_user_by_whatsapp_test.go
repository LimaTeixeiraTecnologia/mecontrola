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

type UpsertUserByWhatsAppSuite struct {
	suite.Suite
	uowMock     *mocks.UnitOfWorkUser
	factoryMock *mocks.RepositoryFactory
	repoMock    *mocks.UserRepository
	uc          *usecases.UpsertUserByWhatsApp
}

func TestUpsertUserByWhatsApp(t *testing.T) {
	suite.Run(t, new(UpsertUserByWhatsAppSuite))
}

func (s *UpsertUserByWhatsAppSuite) SetupTest() {
	s.uowMock = mocks.NewUnitOfWorkUser(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.repoMock = mocks.NewUserRepository(s.T())
	s.uc = usecases.NewUpsertUserByWhatsApp(s.uowMock, s.factoryMock, noop.NewProvider())
}

func (s *UpsertUserByWhatsAppSuite) validInput() input.UpsertUserByWhatsApp {
	return input.UpsertUserByWhatsApp{
		WhatsAppNumber: "+5511987654321",
		Email:          "user@example.com",
		DisplayName:    "Test User",
	}
}

func (s *UpsertUserByWhatsAppSuite) validWhatsApp() valueobjects.WhatsAppNumber {
	wa, err := valueobjects.NewWhatsAppNumber("+5511987654321")
	s.Require().NoError(err)
	return wa
}

func (s *UpsertUserByWhatsAppSuite) TestCriarNovoUsuario() {
	in := s.validInput()
	wa := s.validWhatsApp()

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	created := entities.New(wa, entities.WithDisplayName("Test User"))
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).Return(created, nil)

	out, err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.NotEmpty(out.ID)
	s.Equal("+5511987654321", out.WhatsAppNumber)
}

func (s *UpsertUserByWhatsAppSuite) TestAtualizarComFWW_DisplayNameVazio() {
	in := s.validInput()
	wa := s.validWhatsApp()
	email, _ := valueobjects.NewEmail("user@example.com")

	existing := entities.New(wa, entities.WithEmail(email))

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(existing, nil)
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).
		Return(entities.New(wa, entities.WithEmail(email), entities.WithDisplayName("Test User")), nil)

	out, err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.Equal("Test User", out.DisplayName)
}

func (s *UpsertUserByWhatsAppSuite) TestPreservarFWW_DisplayNamePopulado() {
	in := s.validInput()
	wa := s.validWhatsApp()
	email, _ := valueobjects.NewEmail("user@example.com")

	existing := entities.New(wa, entities.WithEmail(email), entities.WithDisplayName("Existing Name"))

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(existing, nil)
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).
		Return(entities.New(wa, entities.WithEmail(email), entities.WithDisplayName("Existing Name")), nil)

	out, err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.Equal("Existing Name", out.DisplayName)
}

func (s *UpsertUserByWhatsAppSuite) TestErroPropagadoDeFind() {
	in := s.validInput()
	wa := s.validWhatsApp()
	ioErr := errors.New("connection refused")

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, ioErr)

	_, err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, ioErr))
}

func (s *UpsertUserByWhatsAppSuite) TestErroPropagadoDeUpsert() {
	in := s.validInput()
	wa := s.validWhatsApp()
	upsertErr := errors.New("unique violation")

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).
		Return(entities.User{}, upsertErr)

	_, err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, upsertErr))
}
