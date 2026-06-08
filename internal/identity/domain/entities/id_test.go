package entities_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/identity/domain/entities"
)

type IdSuite struct {
	suite.Suite
}

func TestIdSuite(t *testing.T) {
	suite.Run(t, new(IdSuite))
}

func (s *IdSuite) SetupTest() {}

func (s *IdSuite) TestNewID() {
	type args struct {
		count int
	}

	scenarios := []struct {
		name   string
		args   args
		expect func([]string)
	}{
		{
			name: "deve gerar uuid v4 valido",
			args: args{count: 1},
			expect: func(ids []string) {
				parsed, err := uuid.Parse(ids[0])
				s.Require().NoError(err)
				s.Equal(uuid.Version(4), parsed.Version())
			},
		},
		{
			name: "deve gerar ids distintos",
			args: args{count: 20},
			expect: func(ids []string) {
				seen := make(map[string]struct{}, len(ids))
				for _, id := range ids {
					s.NotEmpty(id)
					_, duplicate := seen[id]
					s.False(duplicate)
					seen[id] = struct{}{}
				}
			},
		},
	}

	for _, scenario := range scenarios {
		s.Run(scenario.name, func() {
			ids := make([]string, 0, scenario.args.count)
			for range scenario.args.count {
				ids = append(ids, entities.NewID())
			}

			scenario.expect(ids)
		})
	}
}
