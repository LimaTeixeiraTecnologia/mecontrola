package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type MutationKindSuite struct {
	suite.Suite
}

func TestMutationKindSuite(t *testing.T) {
	suite.Run(t, new(MutationKindSuite))
}

func (s *MutationKindSuite) TestParseMutationKind() {
	type testCase struct {
		name    string
		input   string
		want    valueobjects.MutationKind
		wantErr bool
	}

	cases := []testCase{
		{name: "create", input: "create", want: valueobjects.MutationKindCreate, wantErr: false},
		{name: "update", input: "update", want: valueobjects.MutationKindUpdate, wantErr: false},
		{name: "delete", input: "delete", want: valueobjects.MutationKindDelete, wantErr: false},
		{name: "desconhecido", input: "upsert", wantErr: true},
		{name: "vazio", input: "", wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.ParseMutationKind(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.want, got)
			s.Equal(tc.input, got.String())
		})
	}
}

func (s *MutationKindSuite) TestIotaOrder() {
	s.Equal(1, int(valueobjects.MutationKindCreate))
	s.Equal(2, int(valueobjects.MutationKindUpdate))
	s.Equal(3, int(valueobjects.MutationKindDelete))
}
