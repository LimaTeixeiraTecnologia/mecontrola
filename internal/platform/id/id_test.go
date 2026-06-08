package id_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/platform/id"
)

type UUIDGeneratorSuite struct {
	suite.Suite
	newGenerator func() id.UUIDGenerator
}

func TestUUIDGeneratorSuite(t *testing.T) {
	suite.Run(t, new(UUIDGeneratorSuite))
}

func (s *UUIDGeneratorSuite) SetupTest() {
	s.newGenerator = id.NewUUIDGenerator
}

func (s *UUIDGeneratorSuite) TestNewID() {
	type args struct {
		calls int
	}

	scenarios := []struct {
		name   string
		args   args
		setup  func()
		expect func([]string)
	}{
		{
			name:  "deve retornar uuid v4 valido",
			args:  args{calls: 1},
			setup: func() {},
			expect: func(ids []string) {
				parsed, err := uuid.Parse(ids[0])
				s.Require().NoError(err)
				s.Equal(uuid.Version(4), parsed.Version())
			},
		},
		{
			name:  "deve retornar ids distintos em multiplas chamadas",
			args:  args{calls: 100},
			setup: func() {},
			expect: func(ids []string) {
				seen := make(map[string]struct{}, len(ids))
				for _, generatedID := range ids {
					_, exists := seen[generatedID]
					s.False(exists)
					seen[generatedID] = struct{}{}
				}
			},
		},
		{
			name:  "deve retornar id nao vazio",
			args:  args{calls: 1},
			setup: func() {},
			expect: func(ids []string) {
				s.NotEmpty(ids[0])
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			scenario.setup()

			generator := s.newGenerator()
			ids := make([]string, 0, scenario.args.calls)
			for range scenario.args.calls {
				ids = append(ids, generator.NewID())
			}

			scenario.expect(ids)
		})
	}
}
