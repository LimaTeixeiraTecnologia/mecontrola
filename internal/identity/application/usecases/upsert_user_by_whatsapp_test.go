package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/output"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type UpsertUserByWhatsAppSuite struct {
	suite.Suite
	ctx         context.Context
	obs         observability.Observability
	uowMock     *mocks.UnitOfWorkUser
	factoryMock *mocks.RepositoryFactory
	repoMock    *mocks.UserRepository
}

func TestUpsertUserByWhatsApp(t *testing.T) {
	suite.Run(t, new(UpsertUserByWhatsAppSuite))
}

func (s *UpsertUserByWhatsAppSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.uowMock = mocks.NewUnitOfWorkUser(s.T())
	s.factoryMock = mocks.NewRepositoryFactory(s.T())
	s.repoMock = mocks.NewUserRepository(s.T())
}

func (s *UpsertUserByWhatsAppSuite) validInput() input.UpsertUserByWhatsApp {
	return input.UpsertUserByWhatsApp{
		WhatsAppNumber: "+5511987654321",
		Email:          "user@example.com",
		DisplayName:    "Test User",
	}
}

func (s *UpsertUserByWhatsAppSuite) validWhatsApp() valueobjects.WhatsAppNumber {
	whatsApp, err := valueobjects.NewWhatsAppNumber("+5511987654321")
	s.Require().NoError(err)
	return whatsApp
}

func (s *UpsertUserByWhatsAppSuite) validEmail() valueobjects.Email {
	email, err := valueobjects.NewEmail("user@example.com")
	s.Require().NoError(err)
	return email
}

func (s *UpsertUserByWhatsAppSuite) hydrateDeletedUser(id string, deletedAt time.Time) entities.User {
	user, err := entities.Hydrate(
		id,
		s.validWhatsApp().String(),
		"",
		"",
		string(entities.StatusDeleted),
		deletedAt.Add(-time.Hour),
		deletedAt,
		deletedAt,
	)
	s.Require().NoError(err)
	return user
}

func (s *UpsertUserByWhatsAppSuite) TestExecute() {
	type args struct {
		input input.UpsertUserByWhatsApp
	}

	type dependencies struct {
		factoryMock *mocks.RepositoryFactory
	}

	findErr := errors.New("connection refused")
	upsertErr := errors.New("unique violation")

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(output.UpsertUserByWhatsApp, error)
	}{
		{
			name: "deve criar novo usuario quando whatsapp for inedito",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					email := s.validEmail()
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumberIncludingDeleted(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().
						UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
						RunAndReturn(func(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
							s.Equal(whatsApp.String(), candidate.WhatsApp().String())
							s.Equal(email.String(), candidate.Email().String())
							s.Equal("Test User", candidate.DisplayName())
							s.False(now.IsZero())
							return candidate, nil
						}).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().NoError(err)
				s.NotEmpty(out.ID)
				s.Equal("+5511987654321", out.WhatsAppNumber)
				s.Equal("user@example.com", out.Email)
				s.Equal("Test User", out.DisplayName)
			},
		},
		{
			name: "deve atualizar display name quando usuario existir sem nome",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					email := s.validEmail()
					existing := entities.New(whatsApp, entities.WithEmail(email))
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(existing, nil).Once()
					s.repoMock.EXPECT().
						UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
						RunAndReturn(func(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
							s.Equal("Test User", candidate.DisplayName())
							s.Equal(email.String(), candidate.Email().String())
							s.False(now.IsZero())
							return candidate, nil
						}).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().NoError(err)
				s.Equal("Test User", out.DisplayName)
				s.Equal("user@example.com", out.Email)
			},
		},
		{
			name: "deve preservar display name quando usuario ja estiver preenchido",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					email := s.validEmail()
					existing := entities.New(whatsApp, entities.WithEmail(email), entities.WithDisplayName("Existing Name"))
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(existing, nil).Once()
					s.repoMock.EXPECT().
						UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
						RunAndReturn(func(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
							s.Equal("Existing Name", candidate.DisplayName())
							s.Equal(email.String(), candidate.Email().String())
							s.False(now.IsZero())
							return candidate, nil
						}).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().NoError(err)
				s.Equal("Existing Name", out.DisplayName)
			},
		},
		{
			name: "deve propagar erro ao buscar usuario ativo",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, findErr).Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().Error(err)
				s.ErrorIs(err, findErr)
				s.Empty(out.ID)
			},
		},
		{
			name: "deve propagar erro ao persistir novo usuario",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumberIncludingDeleted(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().
						UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
						Return(entities.User{}, upsertErr).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().Error(err)
				s.ErrorIs(err, upsertErr)
				s.Contains(err.Error(), "unique violation")
				s.Empty(out.ID)
			},
		},
		{
			name: "deve reanimar usuario dentro da janela preservando uuid",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					deletedAt := time.Now().UTC().Add(-10 * 24 * time.Hour)
					deletedUser := s.hydrateDeletedUser("original-uuid", deletedAt)
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumberIncludingDeleted(mock.Anything, whatsApp).Return(deletedUser, nil).Once()
					s.repoMock.EXPECT().
						Reanimate(mock.Anything, mock.Anything, mock.Anything).
						RunAndReturn(func(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
							s.Equal("original-uuid", candidate.ID())
							s.Equal(entities.StatusActive, candidate.Status())
							s.True(candidate.DeletedAt().IsZero())
							s.Equal("user@example.com", candidate.Email().String())
							s.Equal("Test User", candidate.DisplayName())
							s.False(now.IsZero())
							return candidate, nil
						}).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().NoError(err)
				s.Equal("original-uuid", out.ID)
				s.Equal("ACTIVE", out.Status)
				s.Equal("user@example.com", out.Email)
				s.Equal("Test User", out.DisplayName)
			},
		},
		{
			name: "deve criar novo usuario quando registro deletado estiver fora da janela",
			args: args{input: s.validInput()},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					expiredUser := s.hydrateDeletedUser("original-uuid", time.Now().UTC().Add(-(domain.ReanimationWindow + time.Hour)))
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumberIncludingDeleted(mock.Anything, whatsApp).Return(expiredUser, nil).Once()
					s.repoMock.EXPECT().
						UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
						RunAndReturn(func(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
							s.NotEqual("original-uuid", candidate.ID())
							s.Equal("Test User", candidate.DisplayName())
							s.Equal("user@example.com", candidate.Email().String())
							s.False(now.IsZero())
							return candidate, nil
						}).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().NoError(err)
				s.NotEqual("original-uuid", out.ID)
			},
		},
		{
			name: "deve retornar erro para email invalido",
			args: args{
				input: input.UpsertUserByWhatsApp{
					WhatsAppNumber: "+5511987654321",
					DisplayName:    "Test User",
					Email:          "not-an-email",
				},
			},
			dependencies: dependencies{factoryMock: s.factoryMock},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().Error(err)
				s.ErrorIs(err, valueobjects.ErrEmailInvalid)
				s.Empty(out.ID)
			},
		},
		{
			name: "deve aceitar email vazio ao criar usuario",
			args: args{
				input: input.UpsertUserByWhatsApp{
					WhatsAppNumber: "+5511987654321",
					DisplayName:    "Test User",
				},
			},
			dependencies: dependencies{
				factoryMock: func() *mocks.RepositoryFactory {
					whatsApp := s.validWhatsApp()
					s.factoryMock.EXPECT().UserRepository(mock.Anything).Return(s.repoMock).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().FindByWhatsAppNumberIncludingDeleted(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					s.repoMock.EXPECT().
						UpsertByWhatsAppNumber(mock.Anything, mock.Anything, mock.Anything).
						RunAndReturn(func(ctx context.Context, candidate entities.User, now time.Time) (entities.User, error) {
							s.Equal("Test User", candidate.DisplayName())
							s.Empty(candidate.Email().String())
							s.False(now.IsZero())
							return candidate, nil
						}).
						Once()
					return s.factoryMock
				}(),
			},
			expect: func(out output.UpsertUserByWhatsApp, err error) {
				s.Require().NoError(err)
				s.Equal("Test User", out.DisplayName)
				s.Empty(out.Email)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewUpsertUserByWhatsApp(s.uowMock, scenario.dependencies.factoryMock, s.obs)
			out, err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(out, err)
		})
	}
}
