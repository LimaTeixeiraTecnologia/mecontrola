package usecases

import (
	"context"
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability"
	"github.com/JailtonJunior94/devkit-go/pkg/observability/fake"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	application "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/dtos/input"
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type FindUserByWhatsAppSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	repoMock *interfacesmocks.MockUserRepository
}

func TestFindUserByWhatsApp(t *testing.T) {
	suite.Run(t, new(FindUserByWhatsAppSuite))
}

func (s *FindUserByWhatsAppSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repoMock = interfacesmocks.NewMockUserRepository(s.T())
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
		repoMock *interfacesmocks.MockUserRepository
	}

	scenarios := []struct {
		name         string
		args         args
		dependencies dependencies
		expect       func(outputErr error, outputID string)
	}{
		{
			name: "deve retornar usuario encontrado",
			args: args{input: input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"}},
			dependencies: dependencies{
				repoMock: func() *interfacesmocks.MockUserRepository {
					whatsApp := s.mustWhatsApp("+5511987654321")
					user := entities.New(whatsApp)
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(user, nil).Once()
					return s.repoMock
				}(),
			},
			expect: func(outputErr error, outputID string) {
				s.Require().NoError(outputErr)
				s.NotEmpty(outputID)
			},
		},
		{
			name: "deve propagar erro de usuario nao encontrado",
			args: args{input: input.FindUserByWhatsApp{WhatsAppNumber: "+5511987654321"}},
			dependencies: dependencies{
				repoMock: func() *interfacesmocks.MockUserRepository {
					whatsApp := s.mustWhatsApp("+5511987654321")
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, application.ErrUserNotFound).Once()
					return s.repoMock
				}(),
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
			dependencies: dependencies{
				repoMock: func() *interfacesmocks.MockUserRepository {
					whatsApp := s.mustWhatsApp("+5511987654321")
					ioErr := errors.New("db unavailable")
					s.repoMock.EXPECT().FindByWhatsAppNumber(mock.Anything, whatsApp).Return(entities.User{}, ioErr).Once()
					return s.repoMock
				}(),
			},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
				s.Contains(outputErr.Error(), "db unavailable")
				s.Empty(outputID)
			},
		},
		{
			name:         "deve retornar erro ao parsear whatsapp invalido",
			args:         args{input: input.FindUserByWhatsApp{WhatsAppNumber: "not-a-number"}},
			dependencies: dependencies{repoMock: s.repoMock},
			expect: func(outputErr error, outputID string) {
				s.Require().Error(outputErr)
				s.Empty(outputID)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			sut := NewFindUserByWhatsApp(scenario.dependencies.repoMock, s.obs)
			output, err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(err, output.ID)
		})
	}
}
