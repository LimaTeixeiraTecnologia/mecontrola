package usecases_test

import (
	"context"
	"errors"
	"testing"

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

type FindUserByWhatsAppSuite struct {
	suite.Suite
	ctx context.Context
}

func TestFindUserByWhatsApp(t *testing.T) {
	suite.Run(t, new(FindUserByWhatsAppSuite))
}

func (s *FindUserByWhatsAppSuite) SetupTest() {
	s.ctx = context.Background()
}

func (s *FindUserByWhatsAppSuite) mustWhatsApp(raw string) valueobjects.WhatsAppNumber {
	whatsApp, err := valueobjects.NewWhatsAppNumber(raw)
	s.Require().NoError(err)
	return whatsApp
}

func (s *FindUserByWhatsAppSuite) TestExecute() {
	type args struct {
		input input.FindUserByWhatsApp
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
			args: args{input: input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"}},
			setup: func(deps dependencies) {
				whatsApp := s.mustWhatsApp("+5511987654321")
				user := entities.New(whatsApp)
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(user, nil).Once()
			},
			expect: func(outputErr error, outputID string) {
				s.Require().NoError(outputErr)
				s.NotEmpty(outputID)
			},
		},
		{
			name: "deve propagar erro de usuario nao encontrado",
			args: args{input: input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"}},
			setup: func(deps dependencies) {
				whatsApp := s.mustWhatsApp("+5511987654321")
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
			},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
				s.Require().ErrorIs(outputErr, application.ErrUserNotFound)
				s.Empty(outputID)
			},
		},
		{
			name: "deve propagar erro de infraestrutura",
			args: args{input: input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"}},
			setup: func(deps dependencies) {
				whatsApp := s.mustWhatsApp("+5511987654321")
				ioErr := errors.New("db unavailable")
				deps.factory.EXPECT().UserRepository(mock.Anything).Return(deps.repo).Once()
				deps.repo.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, ioErr).Once()
			},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
				s.Contains(outputErr.Error(), "db unavailable")
				s.Empty(outputID)
			},
		},
		{
			name: "deve retornar erro ao parsear whatsapp invalido",
			args: args{input: input.FindUserByWhatsApp{WhatsAppNumber: "not-a-number"}},
			setup: func(deps dependencies) {
				_ = deps
			},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
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

			sut := usecases.NewFindUserByWhatsApp(deps.manager, deps.factory, noop.NewProvider())
			output, err := sut.Execute(s.ctx, scenario.args.input)

			scenario.expect(err, output.ID)
		})
	}
}
