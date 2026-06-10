package valueobjects_test

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/LimaTeixeiraTecnologia/mecontrola/internal/budgets/domain/valueobjects"
)

type ExternalTransactionIDSuite struct {
	suite.Suite
}

func TestExternalTransactionIDSuite(t *testing.T) {
	suite.Run(t, new(ExternalTransactionIDSuite))
}

func (s *ExternalTransactionIDSuite) TestNewExternalTransactionID() {
	type testCase struct {
		name    string
		input   string
		wantErr bool
	}

	cases := []testCase{
		{name: "UUID v4 válido", input: "550e8400-e29b-41d4-a716-446655440000", wantErr: false},
		{name: "UUID v4 válido 2", input: "f47ac10b-58cc-4372-a567-0e02b2c3d479", wantErr: false},
		{name: "ULID válido", input: "01ARZ3NDEKTSV4RRFFQ69G5FAV", wantErr: false},
		{name: "ULID válido 2", input: "01HXYZ1234567890ABCDEFGHJK", wantErr: false},
		{name: "UUID v1 inválido", input: "550e8400-e29b-11d4-a716-446655440000", wantErr: true},
		{name: "UUID uppercase inválido", input: "550E8400-E29B-41D4-A716-446655440000", wantErr: true},
		{name: "vazio", input: "", wantErr: true},
		{name: "aleatório", input: "not-a-valid-id", wantErr: true},
		{name: "ULID lowercase inválido", input: "01arz3ndektsv4rrffq69g5fav", wantErr: true},
	}

	for _, tc := range cases {
		s.Run(tc.name, func() {
			got, err := valueobjects.NewExternalTransactionID(tc.input)
			if tc.wantErr {
				s.Error(err)
				return
			}
			s.NoError(err)
			s.Equal(tc.input, got.String())
		})
	}
}
