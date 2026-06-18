package outbox_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	dbmocks "github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/database/mocks"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/outbox"
)

type FactorySuite struct {
	suite.Suite
}

func TestFactorySuite(t *testing.T) {
	suite.Run(t, new(FactorySuite))
}

func (s *FactorySuite) SetupTest() {}

func (s *FactorySuite) TestOutboxRepository() {
	type args struct{}

	scenarios := []struct {
		name   string
		args   args
		setup  func() *dbmocks.MockDBTX
		expect func(outbox.OutboxRepository)
	}{
		{
			name: "deve criar repositorio nao nil",
			args: args{},
			setup: func() *dbmocks.MockDBTX {
				return dbmocks.NewMockDBTX(s.T())
			},
			expect: func(repository outbox.OutboxRepository) {
				s.NotNil(repository)
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			pool := scenario.setup()

			sut := outbox.NewRepositoryFactory(nil)
			repository := sut.OutboxRepository(pool)

			scenario.expect(repository)
		})
	}
}
