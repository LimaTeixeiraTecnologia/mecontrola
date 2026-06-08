package postgres_test

import (
	"errors"
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	repopostgres "github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories/postgres"
)

type UserRepositorySuite struct {
	suite.Suite
}

func TestUserRepositorySuite(t *testing.T) {
	suite.Run(t, new(UserRepositorySuite))
}

func (s *UserRepositorySuite) SetupTest() {}

func (s *UserRepositorySuite) TestRepositoryConstructionAndSentinels() {
	type args struct {
		wrapped error
		target  error
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(interfaces.UserRepository, args)
	}{
		{
			name: "deve criar repositorio nao nulo",
			args: args{},
			expect: func(repo interfaces.UserRepository, current args) {
				_ = current
				s.NotNil(repo)
			},
		},
		{
			name: "deve reconhecer err user not found com errors join",
			args: args{wrapped: errors.Join(application.ErrUserNotFound), target: application.ErrUserNotFound},
			expect: func(repo interfaces.UserRepository, current args) {
				s.NotNil(repo)
				s.True(errors.Is(current.wrapped, current.target))
			},
		},
		{
			name: "deve reconhecer err whatsapp number in use com errors join",
			args: args{wrapped: errors.Join(application.ErrWhatsAppNumberInUse), target: application.ErrWhatsAppNumberInUse},
			expect: func(repo interfaces.UserRepository, current args) {
				s.NotNil(repo)
				s.True(errors.Is(current.wrapped, current.target))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			repo := repopostgres.NewUserRepository(noop.NewProvider(), nil)
			scenario.expect(repo, scenario.args)
		})
	}
}
