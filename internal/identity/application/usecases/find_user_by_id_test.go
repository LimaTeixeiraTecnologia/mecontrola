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
	interfacesmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces/mocks"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/valueobjects"
)

type FindUserByIDSuite struct {
	suite.Suite
	ctx      context.Context
	obs      observability.Observability
	repoMock *interfacesmocks.MockUserRepository
}

func TestFindUserByID(t *testing.T) {
	suite.Run(t, new(FindUserByIDSuite))
}

func (s *FindUserByIDSuite) SetupTest() {
	s.obs = fake.NewProvider()
	s.ctx = context.Background()
	s.repoMock = interfacesmocks.NewMockUserRepository(s.T())
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
			args: args{input: input.FindUserByID{ID: "00000000-0000-0000-0000-000000000001"}},
			dependencies: dependencies{
				repoMock: func() *interfacesmocks.MockUserRepository {
					whatsApp := s.mustWhatsApp("+5511987654321")
					user, err := entities.Hydrate(
						"00000000-0000-0000-0000-000000000001",
						whatsApp.String(),
						"",
						"Test User",
						string(entities.StatusActive),
						time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
						time.Time{},
					)
					s.Require().NoError(err)
					s.repoMock.EXPECT().FindByID(mock.Anything, "00000000-0000-0000-0000-000000000001").Return(user, nil).Once()
					return s.repoMock
				}(),
			},
			expect: func(outputErr error, outputID string) {
				s.Require().NoError(outputErr)
				s.Equal("00000000-0000-0000-0000-000000000001", outputID)
			},
		},
		{
			name: "deve propagar erro de usuario nao encontrado",
			args: args{input: input.FindUserByID{ID: "00000000-0000-0000-0000-000000000002"}},
			dependencies: dependencies{
				repoMock: func() *interfacesmocks.MockUserRepository {
					s.repoMock.EXPECT().FindByID(mock.Anything, "00000000-0000-0000-0000-000000000002").Return(entities.User{}, application.ErrUserNotFound).Once()
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
			args: args{input: input.FindUserByID{ID: "00000000-0000-0000-0000-000000000003"}},
			dependencies: dependencies{
				repoMock: func() *interfacesmocks.MockUserRepository {
					ioErr := errors.New("db unavailable")
					s.repoMock.EXPECT().FindByID(mock.Anything, "00000000-0000-0000-0000-000000000003").Return(entities.User{}, ioErr).Once()
					return s.repoMock
				}(),
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
			sut := NewFindUserByID(scenario.dependencies.repoMock, s.obs)
			output, err := sut.Execute(s.ctx, scenario.args.input)
			scenario.expect(err, output.ID)
		})
	}
}
