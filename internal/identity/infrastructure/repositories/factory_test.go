package repositories_test

import (
	"testing"

	"github.com/JailtonJunior94/devkit-go/pkg/observability/noop"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/application/interfaces"
	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/infrastructure/repositories"
)

type FactorySuite struct {
	suite.Suite
}

func TestFactorySuite(t *testing.T) {
	suite.Run(t, new(FactorySuite))
}

func (s *FactorySuite) SetupTest() {}

func (s *FactorySuite) TestNewRepositoryFactory() {
	type args struct {
		checkUserRepository bool
	}

	scenarios := []struct {
		name   string
		args   args
		expect func(interfaces.RepositoryFactory)
	}{
		{
			name: "deve criar factory nao nula",
			args: args{},
			expect: func(factory interfaces.RepositoryFactory) {
				s.NotNil(factory)
			},
		},
		{
			name: "deve disponibilizar repositorio de usuario",
			args: args{checkUserRepository: true},
			expect: func(factory interfaces.RepositoryFactory) {
				s.NotNil(factory.UserRepository(nil))
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			factory := repositories.NewRepositoryFactory(noop.NewProvider())
			scenario.expect(factory)
		})
	}
}
