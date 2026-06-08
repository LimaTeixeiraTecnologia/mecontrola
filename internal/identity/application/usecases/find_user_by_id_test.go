package usecases_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases"
	usecasemocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/usecases/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type FindUserByIDSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFindUserByID(t *testing.T) {
	suite.Run(t, new(FindUserByIDSuite))
}

func (s *FindUserByIDSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FindUserByIDSuite) mustWhatsApp(raw string) valueobjects.WhatsAppNumber {
	whatsApp, err := valueobjects.NewWhatsAppNumber(raw)
	s.Require().NoError(err)
	return whatsApp
}

func (s *FindUserByIDSuite) TestExecute() {
	type args struct {
		input input.FindUserByID
	}

	type dependencies struct {
		manager *usecasemocks.FakeManager
		factory *interfacesmocks.MockRepositoryFactory
		repo    *interfacesmocks.MockUserRepository
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func(dependencies)
		expect func(outputErr error, outputID string)
	}{
		{
			name: "deve retornar usuario encontrado",
			args: args{input: input.FindUserByID{ID: "existing-user-id"}},
			setup: func(deps dependencies) {
				whatsApp := s.mustWhatsApp("+5511987654321")
				user, err := entities.Hydrate(
					"existing-user-id",
					whatsApp.String(),
					"",
					"Test User",
					string(entities.StatusActive),
					time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
					time.Time{},
				)
				s.Require().NoError(err)
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByID(mock.Anything, "existing-user-id").Return(user, nil).Once()
			},
			expect: func(outputErr error, outputID string) {
				s.Require().NoError(outputErr)
				s.Equal("existing-user-id", outputID)
			},
		},
		{
			name: "deve propagar erro de usuario nao encontrado",
			args: args{input: input.FindUserByID{ID: "non-existent"}},
			setup: func(deps dependencies) {
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByID(mock.Anything, "non-existent").Return(entities.User{}, application.ErrUserNotFound).Once()
			},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
				s.Require().ErrorIs(outputErr, application.ErrUserNotFound)
				s.Empty(outputID)
			},
		},
		{
			name: "deve propagar erro de infraestrutura",
			args: args{input: input.FindUserByID{ID: "some-id"}},
			setup: func(deps dependencies) {
				ioErr := errors.New("db unavailable")
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByID(mock.Anything, "some-id").Return(entities.User{}, ioErr).Once()
			},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
				s.Contains(outputErr.Error(), "db unavailable")
				s.Empty(outputID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			deps := dependencies{
				manager: usecasemocks.NewFakeManager(),
				factory: interfacesmocks.NewMockRepositoryFactory(s.T()),
				repo:    interfacesmocks.NewMockUserRepository(s.T()),
			}
			scenario.setup(deps)

			sut := usecases.NewFindUserByID(deps.manager, deps.factory, noop.NewProvider())
			output, err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err, output.ID)
		})
	}
}
