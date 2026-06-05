package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
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

func (s *UpsertUserByWhatsAppSuite) TestCriarNovoQuandoNumeroInedito() {
	in := s.validInput()
	wa := s.validWhatsApp()

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	s.repoMock.On("FindByWhatsAppNumberIncludingDeleted", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
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
	s.repoMock.On("FindByWhatsAppNumberIncludingDeleted", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).
		Return(entities.User{}, upsertErr)

	_, err := s.uc.Execute(context.Background(), in)
	s.Require().Error(err)
	s.True(errors.Is(err, upsertErr))
}

func (s *UpsertUserByWhatsAppSuite) TestReanimateDentroDaJanela() {
	in := s.validInput()
	wa := s.validWhatsApp()

	deletedBase := time.Now().UTC().Add(-10 * 24 * time.Hour)
	deletedUser, hydErr := entities.Hydrate(
		"original-uuid", wa.String(), "", "",
		string(entities.StatusDeleted),
		deletedBase.Add(-1*time.Hour), deletedBase, deletedBase,
	)
	s.Require().NoError(hydErr)

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	s.repoMock.On("FindByWhatsAppNumberIncludingDeleted", mock.Anything, wa).Return(deletedUser, nil)

	expected, hydErr2 := entities.Hydrate("original-uuid", wa.String(), "user@example.com", "Test User",
		string(entities.StatusActive), deletedBase.Add(-1*time.Hour), time.Now().UTC(), time.Time{})
	s.Require().NoError(hydErr2)
	s.repoMock.On("Reanimate", mock.Anything, mock.Anything, mock.Anything).Return(expected, nil)

	out, err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.Equal("original-uuid", out.ID)
	s.Equal("ACTIVE", out.Status)
}

func (s *UpsertUserByWhatsAppSuite) TestForaDaJanelaCriaNovo() {
	in := s.validInput()
	wa := s.validWhatsApp()

	expiredAt := time.Now().UTC().Add(-(domain.ReanimationWindow + time.Hour))
	expiredUser, hydErr := entities.Hydrate(
		"original-uuid", wa.String(), "", "",
		string(entities.StatusDeleted),
		expiredAt.Add(-1*time.Hour), expiredAt, expiredAt,
	)
	s.Require().NoError(hydErr)

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	s.repoMock.On("FindByWhatsAppNumberIncludingDeleted", mock.Anything, wa).Return(expiredUser, nil)

	created := entities.New(wa, entities.WithDisplayName("Test User"))
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).Return(created, nil)

	out, err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.NotEqual("original-uuid", out.ID)
}

func (s *UpsertUserByWhatsAppSuite) TestEmailInvalidoRetornaErro() {
	_, err := s.uc.Execute(context.Background(), input.UpsertUserByWhatsApp{
		WhatsAppNumber: "+5511987654321",
		Email:          "not-an-email",
	})
	s.Require().Error(err)
}

func (s *UpsertUserByWhatsAppSuite) TestEmailVazioNaoErra() {
	in := input.UpsertUserByWhatsApp{
		WhatsAppNumber: "+5511987654321",
		Email:          "",
		DisplayName:    "Test User",
	}
	wa := s.validWhatsApp()

	s.factoryMock.On("UserRepository", mock.Anything).Return(s.repoMock)
	s.repoMock.On("FindByWhatsAppNumber", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	s.repoMock.On("FindByWhatsAppNumberIncludingDeleted", mock.Anything, wa).Return(entities.User{}, application.ErrUserNotFound)
	created := entities.New(wa, entities.WithDisplayName("Test User"))
	s.repoMock.On("UpsertByWhatsAppNumber", mock.Anything, mock.Anything, mock.Anything).Return(created, nil)

	out, err := s.uc.Execute(context.Background(), in)
	s.Require().NoError(err)
	s.Equal("Test User", out.DisplayName)
}
